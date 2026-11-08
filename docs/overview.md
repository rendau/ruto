# Обзор проекта

**ruto** — управляемый API-gateway. Система состоит из двух бэкендов на Go и админ-панели на Vue 3.

- **`core`** — control plane (плоскость управления). Хранит конфигурацию в PostgreSQL, отдаёт admin-API (gRPC + grpc-gateway REST), раздаёт админ-SPA и публикует **snapshot** конфигурации.
- **`gateway`** — data plane (плоскость данных). Не имеет своей БД: периодически тянет snapshot из `core`, на его основе строит маршруты и проксирует входящий HTTP/gRPC трафик на реальные backend-сервисы, применяя auth, CORS, метрики и подстановку переменных.
- **`apps/admin`** — Vue 3 SPA, фронтенд для `core` (через его REST `/api/*`).

```
                 admin SPA  ─────┐
                                 ▼
   ┌──────────────────────────────────────┐        snapshot (poll /10s)
   │  core  (control plane)               │ ──────────────────────────┐
   │  Postgres • admin-API • SPA • snapshot│                           │
   └──────────────────────────────────────┘                           ▼
                                                   ┌──────────────────────────────┐
   внешний клиент  ── HTTP/gRPC ───────────────▶   │  gateway  (data plane / proxy)│
                                                   └──────────────────────────────┘
                                                                   │ proxy
                                                                   ▼
                                                          backend-сервисы
```

## Ключевые понятия

| Термин | Смысл |
| --- | --- |
| **Root** | Корневая конфигурация: общий auth, CORS (`allow_origins`: точный origin `https://app.example.com`, поддомены `https://*.example.com`, либо `*`; с `allow_credentials` звёздочка в origins, methods и headers отдаётся браузеру как то, что он запросил, потому что буквальную `*` с куками браузер не принимает; `max_age` — секунды или длительность вида `24h`, хранится как написано, в заголовок `Access-Control-Max-Age` уходит в секундах), JWT-провайдеры, глобальные переменные. Провайдер — имя и адрес JWK; с `issuer` это OpenID Connect сервер (адреса из discovery, `jwk_url` не нужен), через который gateway сам логинит пользователей: `internal_url` для серверных запросов, `public_url` для адреса возврата (по умолчанию `base_url`) и `client` — клиент по умолчанию (`id`, `secret`, `scopes`). Хранится как одна строка `root` (jsonb). |
| **App** | Приложение/backend: `path_prefix`, backend URL, gRPC-порт, свой auth, переменные, набор endpoint'ов. Необязательное поле `group` — произвольная текстовая метка, по которой админка группирует приложения в боковой панели (на маршрутизацию не влияет). Список `jwt` — провайдеры, через которых приложение логинит пользователей **своим** клиентом (отдельный вход `/_auth/<провайдер>/<префикс>/…` и своя кука на своём пути) вместо общего клиента провайдера. Префикс `_auth` занят маршрутами входа. |
| **Endpoint** | Конкретный маршрут внутри App: HTTP (метод + путь) или gRPC (service/method/path), auth, переопределение backend-пути и заголовков. Путь может заканчиваться на `/*`, если у App включён `allow_wildcard`. |
| **Snapshot** | Денормализованный, «запечённый» слепок `root + apps + endpoints` с хэшем-версией. Именно его потребляет `gateway`. |
| **Auth** | Набор методов проверки: `basic`, `api_key`, `jwt`, `ip_validation`. Наследуется Root → App → Endpoint в режиме `extend`/`replace`. Метод `jwt` ссылается на провайдера из Root по имени (`provider`) и принимает любой его ключ; `kid` — необязательная привязка к одному ключу. Без `provider` подходит ключ любого провайдера. Токен берётся из `Authorization: Bearer`, query-параметра `auth_token` или, у метода с `provider`, из куки `ruto_<provider>` (у приложения со своим клиентом — `ruto_<provider>_<префикс>`); принятый из куки токен шлюз дополнительно отдаёт backend'у как `Authorization: Bearer`, саму куку не трогает. Роль `клиент:роль` ищется только среди ролей этого клиента (`resource_access.<клиент>.roles`), роль без клиента — в `roles`, `role`, `realm_access.roles` и у любого клиента. Куки выдаёт сам шлюз на маршрутах `/_auth/<provider>/login`, `callback`, `logout` (вход по коду с PKCE через любой OpenID Connect провайдер) и обновляет на `POST /_auth/<provider>/refresh` по refresh-токену — фронт зовёт его, получив 401 (см. `architecture.md` и `sso-plan.md`). Свои куки входа (`ruto-rt_…`, `ruto-login_…`) шлюз бэкендам не передаёт. |
| **Vars (переменные)** | Пары ключ-значение, наследуются вниз и подставляются (`interpolate`) в backend-заголовки, query-параметры и auth. |
| **Compression (сжатие)** | Что просить у backend'а (без сжатия или как просит клиент) и как сжимать ответ клиенту: алгоритмы по приоритету (`br`, `zstd`, `gzip`), порог размера, типы содержимого. Наследуется Root → App → Endpoint: пустое поле берётся у родителя, у Root пустое заполняется значениями по умолчанию. |
| **Session** | Админ-сессия (JWT, секрет `ADMIN_JWT_SECRET`), прокидывается через `context`; на ней строится авторизация в usecase-слое. |

## Иерархия конфигурации

`Root` владеет списком `App`, каждый `App` владеет списком `Endpoint`. Auth, логирование, сжатие и переменные **наследуются сверху вниз** при сборке snapshot:

```
Root ──(inherit)──▶ App ──(inherit)──▶ Endpoint
 auth, logging,      auth, logging,      auth, logging,
 compression, vars   compression, vars   compression, vars
```

В БД `app` и `endpoint` хранятся отдельными таблицами (jsonb `data`), но в snapshot они вложены в дерево `Root.Apps[].Endpoints[]`.

## Карта репозитория

```
cmd/                  entrypoints: core/main.go, gateway/main.go
internal/
  app/                composition root + lifecycle (Init/Start/Wait/Stop/Exit)
    core/             сборка core: pgpool, миграции, сервисы, grpc, grpc-gateway, SPA, system-сервер (healthcheck, pulse)
    gateway/          сборка gateway: gw-сервис + system-сервер (healthcheck, metrics)
    common/           logger, перехват сигналов
  config/             разбор env (caarlos0/env) — core/ и gateway/
  constant/           константы (имя сервиса, режимы auth, JWT-алгоритмы)
  errs/               sentinel-ошибки (тип Err string) + ErrFull
  domain/             доменный слой: <entity>/{model,service,repo}
    root, app, endpoint, usr, snapshot, session, auth, vars, common,
    apikey, changerequest, secretref (маскирование секретов для MCP)
  usecase/            прикладной слой: оркестрация + авторизация
    root, app, endpoint, usr, snapshot, stats, gateway, apikey, changerequest,
    system (состояние ruto для служебного порта, без проверки входа)
  handler/grpc/       транспорт core: gRPC-хендлеры + dto (encode/decode proto↔domain)
  handler/mcp/        встроенный MCP-сервер для AI-агентов (см. mcp-server.md)
  handler/system/     служебный порт core: манифест pulse, маршруты для pulse (см. pulse.md)
  service/            вспомогательные сервисы
    cache/            кэш (redis | mem) за общим интерфейсом
    swagger/          парсинг swagger backend'ов
    grpcreflect/      gRPC reflection
    gw/               ВЕСЬ data plane gateway (см. architecture.md)
  infra/httpx/        фабрика http-клиентов с таймаутами и лимитами (все исходящие клиенты — через неё)
  infra/metrics/      prometheus-метрики
  infra/pulsekit/     стандарт манифеста pulse (копия из репозитория pulse, не правится)
api/proto/ruto_v1/    ИСХОДНЫЕ .proto (редактировать здесь)
pkg/proto/ruto_v1/    СГЕНЕРИРОВАННЫЙ код (не редактировать вручную)
migrations/           SQL-миграции (golang-migrate)
config/               *.env.example шаблоны
docker/               Dockerfile'ы для core и gateway
docs/                 эта документация
apps/admin/           Vue 3 SPA
```

## Технологический стек

- **Go 1.26**, gRPC + [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) (REST поверх gRPC).
- **PostgreSQL** через `pgx/v5` (pgxpool), query-builder `squirrel`, тонкая ORM-обёртка [`mobone`](https://github.com/rendau/mobone).
- **Redis** (`go-redis`) опционально для кэша; иначе in-memory.
- HTTP-роутинг gateway — `go-chi`, gRPC-прокси — `mwitkow/grpc-proxy`.
- JWT — `golang-jwt/jwt/v5`; миграции — `golang-migrate`; метрики — `prometheus`.
- Утилиты — `samber/lo`; конфиг — `caarlos0/env`; тесты — `stretchr/testify`.
- Фронтенд — Vue 3, Pinia, Vue Router, naive-ui, Vite, pnpm.

Подробнее: [architecture.md](architecture.md) · [code-style.md](code-style.md) · [onboarding.md](onboarding.md).
