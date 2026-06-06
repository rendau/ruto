# Архитектура

Проект следует **слоистой (clean / hexagonal) архитектуре** с явным разделением на control plane (`core`) и data plane (`gateway`). Зависимости направлены внутрь: транспорт → usecase → domain. Внешние детали (БД, gRPC, кэш) подключаются через интерфейсы, объявленные **на стороне потребителя**.

## Слои `core`

```
   gRPC / grpc-gateway (REST /api/*)          internal/handler/grpc/*  + dto/*
            │  proto-DTO ↔ domain
            ▼
   Usecase (оркестрация + авторизация)        internal/usecase/*
            │  domain-модели, интерфейсы зависимостей
            ▼
   Domain Service (бизнес-правила)            internal/domain/<e>/service/*
            │  RepoDbI
            ▼
   Repo (persistence)                         internal/domain/<e>/repo/db/*
            │  mobone.ModelStore + squirrel
            ▼
         PostgreSQL (pgxpool)
```

Назначение слоёв:

- **handler/grpc** — тонкий транспорт. Принимает proto-сообщение, через `dto.Decode*` превращает в domain-модель, зовёт usecase, через `dto.Encode*` отдаёт proto-ответ. Никакой бизнес-логики. Ошибки возвращает как есть — их маппит interceptor.
- **usecase** — прикладная логика: проверки авторизации по session (`CtxIsAuthorized`, `CtxIsAdmin`), валидация запроса, оркестрация нескольких domain-сервисов. Каждый usecase объявляет нужные ему интерфейсы в своём `interfaces.go`.
- **domain/service** — бизнес-правила сущности и единственная точка работы с репозиторием. Тоже объявляет `RepoDbI` у себя.
- **domain/model** — сущности + чистые преобразования: `Normalize`, `InheritDown`, `Interpolate`, `Merge` (см. ниже).
- **domain/repo/db** — реализация хранилища: маппинг domain ↔ repo-модели (`Decode*`/`Encode*`), запросы через `mobone.ModelStore`.

## Слои `gateway` (data plane)

Весь gateway живёт в `internal/service/gw/`:

```
internal/service/gw/
  gw.go                 Service — корень: связывает core_client, http- и grpc-серверы
  core_client/          gRPC-клиент к core: поллинг snapshot + heartbeat
  server/http/          обёртка http.Server с горячей заменой handler'а
  server/grpc/          обёртка grpc-прокси-сервера с горячей заменой handler'а
  handler/http/         сборка chi-роутера из snapshot
    middleware/         cors, auth, backend_request_params, metrics, request_log, chain
    proxy/              httputil.ReverseProxy на backend
  handler/grpc/         маршрутизация gRPC по (x-ruto-app-name + method path) → backend
  service/auth/         движок авторизации (методы basic/api_key/jwt/ip_validation)
    authorizer/         реализации каждого метода проверки
  service/jwk/          загрузка и кэш JWK (ключи для JWT)
  service/log, metrics  кросс-срезовые сервисы
```

Ключевая идея: **handler'ы пересобираются целиком** при каждом новом snapshot и атомарно подменяются в серверах (`SetHandler`). Сервер слушает порт постоянно, а маршрутная таблица меняется на лету без рестарта.

## Поток HTTP-запроса через gateway

```
клиент → http.Server → chi-router (по snapshot)
       → middleware.Chain:
            Metrics            учёт запроса в prometheus
            RequestLog         access-log (если включён)
            Auth               проверка методов авторизации endpoint'а
            BackendRequestParams  подстановка заголовков/query из vars
       → proxy.ReverseProxy → backend (app.Backend.Url [+ endpoint.Backend.CustomPath])
```

Сборка роутера — `gw/handler/http/service.go:buildHandler`: для каждого активного App берётся `PathPrefix`, для каждого активного HTTP-endpoint строится путь `path_prefix[/http.path]`, оборачивается в цепочку middleware и регистрируется в chi (метод `*` → любой метод). Сверху навешивается CORS из `Root.Cors`.

## Поток gRPC-запроса через gateway

Маршрут выбирается по двум значениям из вызова: metadata-заголовку **`x-ruto-app-name`** и полному пути метода `/package.Service/Method`. Подробности и примеры клиентов — в [gateway-grpc.md](gateway-grpc.md).

## Snapshot: как конфиг попадает из core в gateway

Snapshot — это центральный механизм синхронизации. Контракт — gRPC-сервисы `Snapshot` и `Gateway`.

**На стороне core** (`internal/usecase/snapshot`):
1. `construct` — собирает `Root` + все `Apps` + их `Endpoints` в одно дерево.
2. `Normalize` — приведение к валидному виду.
3. Сериализация в jsonb, `sha256` от содержимого → **версия** (hash).
4. Сохранение в таблицу `snapshot`. Отдаётся через `Snapshot.GetVersion` / `Snapshot.Get`.

**На стороне gateway** (`internal/service/gw/core_client`) — три независимые горутины:
1. **`refreshWorker`** — единственная горутина, выполняющая `refresh()` (проверка версии + применение). Делает **одну** проверку при старте, дальше реагирует только на триггеры из `triggerCh`. **Периодического опроса версии нет.** Единственный потребитель `triggerCh` гарантирует, что проверки версии и применение snapshot **никогда не идут параллельно**.
2. `refresh()` шлёт `GetVersion`; если версия не изменилась — выходит. Если изменилась — `Get`, декодирует в `rootModel.Root`, зовёт `onConfig` (= `gw.Service.SetConfig`).
3. **`heartbeatWorker`** — раз в `HeartbeatInterval` (10s) шлёт `Gateway.Heartbeat` (id, host, версия, память, горутины, последняя ошибка) для мониторинга в админке. Полностью независим от refresh: это только liveness/телеметрия.
4. **`subscribeWorker`** — push-канал (см. ниже).

**Мгновенное уведомление (push).** Распространение конфига — событийное, а не по таймеру. Gateway держит постоянный серверный стрим `Gateway.Subscribe` к core:
- **gw**: `subscribeWorker` открывает стрим и читает уведомления; на каждое — кладёт триггер в `triggerCh` (досрочный `refresh`). При обрыве стрима (рестарт core, сеть) — бесконечно переподключается с задержкой `subscribeReconnectDelay`.
- **core**: хендлер `Subscribe` через usecase регистрирует gateway в пуле `internal/service/gateways` (callback-уведомитель) и блокируется на время жизни стрима. Сразу после (пере)подключения шлёт начальное уведомление — это и есть catch-up после простоя/реконнекта. После успешного `Refresh` snapshot'а usecase зовёт `gateways.NotifyAll()` → каждый callback шлёт уведомление в свой стрим.
- **отключение**: когда gw отпадает, контекст стрима отменяется → хендлер выходит → `defer Unregister` убирает callback из пула. Отправка в стрим сериализована (один потребитель `notifyCh` на стрим), параллельных `Send` нет.
- **graceful shutdown core**: `gRPC GracefulStop` ждёт завершения всех RPC, а `Subscribe` — долгоживущий стрим. Поэтому при остановке core `app.Stop()` зовёт `gatewaysService.Close()` (закрывает `Done()`-канал) → все `Subscribe` сразу выходят → `GracefulStop` завершается мгновенно. Плюс у самого `GrpcServer.Stop()` есть таймаут-фолбэк на жёсткий `Stop()`.

**Конфигурационный pipeline** (`gw.Service.SetConfig`) — три чистых преобразования над `Root`:

```
conf.Normalize()     // привести значения к каноничному виду, валидировать
conf.InheritDown()   // Root.auth/vars → App → Endpoint (Merge + FillMissing)
conf.Interpolate()   // подставить переменные в заголовки/query/auth
```

Затем из готового `Root` собираются новые http- и grpc-handler'ы и атомарно ставятся в серверы; `ready` → `true` (это видит `/healthcheck`). JWK URLs из конфига передаются в `jwk`-сервис.

> Тот же триптих `Normalize → InheritDown → Interpolate` определён на доменных моделях (`root`, `app`, `endpoint`) и переиспользуется. Наследование auth — через `auth.Merge(parent, child)` с учётом режима `extend`/`replace`; наследование переменных — через `Vars.FillMissing(parent)`.

## Жизненный цикл приложения

Оба бэкенда реализуют единый контракт в `internal/app/<name>/app.go`:

```go
a := app.New()
a.Init()   // конфиг, зависимости, серверы — вся сборка (composition root)
a.Start()  // запуск слушателей (неблокирующий)
a.Wait()   // блокировка до SIGINT/SIGTERM (common.WaitSignal)
a.Stop()   // graceful shutdown с таймаутами
a.Exit()   // освобождение ресурсов, код выхода
```

`Init` — это **composition root**: здесь вручную (без DI-фреймворка) создаются repo → service → usecase → handler и регистрируются в gRPC-сервере. См. `internal/app/core/app.go`.

## Обработка ошибок и транспорт

- Доменные/прикладные ошибки — sentinel-значения типа `errs.Err` (`errs.NotAuthorized`, `errs.ObjectNotFound`, …) либо `errs.ErrFull` (с `Desc` и `Fields`).
- Внутри слоёв ошибки **оборачиваются** с именем вызванной функции: `fmt.Errorf("svc.Get: %w", err)`.
- На границе gRPC `GrpcInterceptorError` (`internal/app/core/grpc.go`) превращает ошибку в `status` + `ruto_v1.ErrorRep{Code, Message, Fields}`. Цепочка interceptor'ов core: `CtxWithoutCancel → Session → Recovery → Error`.
- `GrpcInterceptorSession` достаёт Bearer-токен из metadata, парсит session и кладёт в `context`; дальше usecase-слой решает вопросы доступа.

## БД

- `root`, `app`, `endpoint`, `snapshot` — «schemaless»: ключевые поля-колонки + `data jsonb` со всей моделью. Это позволяет менять структуру конфигурации без миграций схемы.
- `usr` — обычная реляционная таблица (id, username, password-hash, флаги).
- Доступ — через `mobone.ModelStore` (List/Get/Create/Update/Delete) и `squirrel` (плейсхолдеры `$N`). Базовый помощник — `internal/domain/common/repo/pg/base.go`.

Дальше: соглашения по коду — [code-style.md](code-style.md); практические рецепты — [onboarding.md](onboarding.md).
