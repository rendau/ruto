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
| **Root** | Корневая конфигурация: общий auth, CORS, JWT (JWK URLs), глобальные переменные. Хранится как одна строка `root` (jsonb). |
| **App** | Приложение/backend: `path_prefix`, backend URL, gRPC-порт, свой auth, переменные, набор endpoint'ов. |
| **Endpoint** | Конкретный маршрут внутри App: HTTP (метод + путь) или gRPC (service/method/path), auth, переопределение backend-пути и заголовков. |
| **Snapshot** | Денормализованный, «запечённый» слепок `root + apps + endpoints` с хэшем-версией. Именно его потребляет `gateway`. |
| **Auth** | Набор методов проверки: `basic`, `api_key`, `jwt`, `ip_validation`. Наследуется Root → App → Endpoint в режиме `extend`/`replace`. |
| **Vars (переменные)** | Пары ключ-значение, наследуются вниз и подставляются (`interpolate`) в backend-заголовки, query-параметры и auth. |
| **Session** | Админ-сессия (JWT, секрет `ADMIN_JWT_SECRET`), прокидывается через `context`; на ней строится авторизация в usecase-слое. |

## Иерархия конфигурации

`Root` владеет списком `App`, каждый `App` владеет списком `Endpoint`. Auth и переменные **наследуются сверху вниз** при сборке snapshot:

```
Root ──(inherit)──▶ App ──(inherit)──▶ Endpoint
 auth, vars          auth, vars          auth, vars
```

В БД `app` и `endpoint` хранятся отдельными таблицами (jsonb `data`), но в snapshot они вложены в дерево `Root.Apps[].Endpoints[]`.

## Карта репозитория

```
cmd/                  entrypoints: core/main.go, gateway/main.go
internal/
  app/                composition root + lifecycle (Init/Start/Wait/Stop/Exit)
    core/             сборка core: pgpool, миграции, сервисы, grpc, grpc-gateway, SPA
    gateway/          сборка gateway: gw-сервис + system-сервер (healthcheck, metrics)
    common/           logger, перехват сигналов
  config/             разбор env (caarlos0/env) — core/ и gateway/
  constant/           константы (имя сервиса, режимы auth, JWT-алгоритмы)
  errs/               sentinel-ошибки (тип Err string) + ErrFull
  domain/             доменный слой: <entity>/{model,service,repo}
    root, app, endpoint, usr, snapshot, session, auth, vars, common
  usecase/            прикладной слой: оркестрация + авторизация
    root, app, endpoint, usr, snapshot, stats, gateway
  handler/grpc/       транспорт core: gRPC-хендлеры + dto (encode/decode proto↔domain)
  service/            вспомогательные сервисы
    cache/            кэш (redis | mem) за общим интерфейсом
    swagger/          парсинг swagger backend'ов
    grpcreflect/      gRPC reflection
    gw/               ВЕСЬ data plane gateway (см. architecture.md)
  infra/metrics/      prometheus-метрики
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
