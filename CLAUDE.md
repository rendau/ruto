# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**ruto** is a managed API gateway split into two Go backends plus a Vue 3 admin SPA:

- **`core`** (control plane) — stores configuration in PostgreSQL, serves the admin API (gRPC + grpc-gateway REST under `/api/*`), serves the admin SPA, and publishes a **snapshot** of the configuration.
- **`gateway`** (data plane) — has no DB; polls the snapshot from `core` every 10s, rebuilds its routing, and reverse-proxies incoming HTTP/gRPC traffic to backend services, applying auth, CORS, metrics, and variable interpolation.
- **`apps/admin`** — Vue 3 SPA frontend for `core`.

Entrypoints: `cmd/core/main.go`, `cmd/gateway/main.go`.

## Deeper docs

This repo has rich docs — read them before non-trivial work:

- `docs/overview.md` — concepts (Root/App/Endpoint/Snapshot/Auth/Vars/Session), repo map, tech stack.
- `docs/architecture.md` — layering, request flows, snapshot pipeline, app lifecycle, error handling.
- `docs/sso.md` — login through the gateway: OpenID Connect provider in Root, `/_auth/…` routes, access/refresh cookies, refresh, `me`, `auth_error`, JS SDK served by the gateway.
- `docs/code-style.md` — conventions.
- `docs/onboarding.md` — local setup and recipes for common changes.
- `docs/gateway-grpc.md` — connecting to the gateway over gRPC.
- `docs/monitoring.md` — admin traffic charts & logs (Prometheus / Loki / Graylog integration).
- `docs/pulse.md` — integration with pulse: the manifest and `/routes` on core's system port (pulse talks to core only, never to the gateways); `internal/infra/pulsekit` is a verbatim copy from the pulse repo.
- `docs/mcp-server.md` — built-in MCP server for AI agents: api keys and OAuth (Claude connectors), masked secrets, change requests approved in the admin.

## Commands

```bash
make run-core         # control plane (gRPC :5050, HTTP :9090, system/metrics :3003)
make run-gateway      # data plane (HTTP/gRPC ports from .env.gateway)
make run-admin        # vite dev server for the SPA
make build-all        # core + gateway binaries → cmd/build/
make build-admin      # build the SPA (pnpm)
make generate-proto   # regenerate pkg/proto from api/proto/ruto_v1/*.proto

go test ./...                          # all tests
go test ./internal/usecase/app/...     # single package
```

Env: copy `config/core.env.example` → `.env.core` and `config/gateway.env.example` → `.env.gateway`. `core` needs at least `ADMIN_JWT_SECRET` and `PG_DSN`, and applies migrations itself on start.

## Architecture essentials

- **Layering (core), dependencies point inward:** `handler/grpc` (thin transport, proto↔domain via `dto.Decode*`/`Encode*`) → `usecase` (authorization + orchestration) → `domain/<entity>/service` (business rules, sole repo entry point) → `domain/<entity>/repo/db` (mobone + squirrel) → PostgreSQL (pgxpool). Each layer declares the interfaces it needs **in its own `interfaces.go`** (consumer-side, `I` suffix). Errors bubble up wrapped (`fmt.Errorf("svc.Get: %w", err)`); sentinel `errs.Err` values are returned **unwrapped** so the gRPC interceptor can map them.

- **Snapshot is the sync mechanism.** core builds `Root + Apps + Endpoints` into one tree, normalizes it, hashes it (sha256 → version), stores it. gateway polls `Snapshot.GetVersion`; on change it fetches and runs the config pipeline `Normalize() → InheritDown() → Interpolate()`, then **rebuilds HTTP/gRPC handlers from scratch and atomically swaps them** into the running servers (`SetHandler`) — no restart. The whole data plane lives in `internal/service/gw/`.

- **Config inheritance:** auth and vars flow `Root → App → Endpoint` (`auth.Merge` with `extend`/`replace`, `Vars.FillMissing`). The same `Normalize/InheritDown/Interpolate` triptych is defined on the domain models and reused.

- **Composition root:** `internal/app/<name>/app.go` wires everything by hand (no DI framework) and runs the `Init → Start → Wait → Stop → Exit` lifecycle. New gRPC **services** get registered here; new **methods** on an existing service do not.

- **DB:** `root`/`app`/`endpoint`/`snapshot` are "schemaless" — key columns + a `data jsonb` holding the full model, so config structure changes usually need only a Go-model edit, no schema migration. `usr` is a normal relational table.

## Gotchas

- Edit only `api/proto/ruto_v1/*.proto`, then `make generate-proto`. Never hand-edit `pkg/proto/ruto_v1/*` (generated).
- Go 1.26+: take pointers to a value/literal/**expression** with `new(expr)` (e.g. `new(int(sv.ToInteger()))`) — never introduce a temp var just to take its address (`tmp := expr; &tmp`), and never use `lo.ToPtr(...)`. Applies project-wide. Exception: when the expression also returns an `error` (`v, err := f()`), the var is needed for error handling, so `&v` is fine.
- All struct methods MUST use pointer receivers (`func (m *T) ...`), even read-only/pure ones — never mix value and pointer receivers on a type.
- Comment functions/code blocks only when logically necessary — to explain a non-obvious "why" (invariant, subtle nuance, rationale). Don't comment self-evident code or restate what it already says.
- Don't add redundant `TrimSpace`/nil checks on already-normalized domain entities; normalization happens once in `Normalize()`.
- In `internal/app/*`, packages are aliased with a `P` suffix (`usecaseAppP`, `handlerGrpcP`) to disambiguate.
## Тестовый стенд

Локальный одноразовый стенд для проверки core + админки (значения — только для локального docker-стенда, не боевые):

- **Postgres:** `docker run --rm -d --name ruto-test-pg -e POSTGRES_USER=ruto -e POSTGRES_PASSWORD=ruto -e POSTGRES_DB=ruto -p 55432:5432 postgres:17`
  - DSN: `postgres://ruto:ruto@localhost:55432/ruto?sslmode=disable`
- **core:** собирать `go build -o <scratchpad>/stand/core cmd/core/main.go` и запускать **из каталога вне репозитория** (чтобы не подхватился `.env.core`), положив рядом симлинк `migrations` → `<repo>/migrations`:
  `PG_DSN=<dsn> ADMIN_JWT_SECRET=ruto-local-test-stand-secret HTTP_PORT=9090 HTTP_CORS=true GRPC_PORT=5050 SYSTEM_PORT=3003 ./core`
  - HTTP `:9090` (REST `/api/*`), gRPC `:5050`, system `:3003` (`/healthcheck`, `/.well-known/pulse`, `/.well-known/pulse/status`, `/diag/gateways`, `/routes`).
  - MCP (для проверок MCP-сервера): добавить `MCP_ENABLED=true MCP_PORT=5060` → `http://localhost:5060/`; ключ `rtk_…` создаётся в админке (Profile → API keys) или `POST /api/api_key {"name": "..."}` с JWT администратора.
  - OAuth для MCP: ещё `MCP_OAUTH_ENABLED=true MCP_PUBLIC_URL=http://localhost:5060 MCP_OAUTH_ADMIN_URL=http://localhost:5173`. Клиента регистрировать с адресом возврата `http://localhost:18099/callback` (loopback разрешён по умолчанию; там никто не слушает — код берётся из адресной строки после «Allow»), страница согласия — `http://localhost:5173/oauth/authorize?...`.
- **gateway (нужен только для проверок трафика):** `go build -o <scratchpad>/stand/gateway cmd/gateway/main.go`, запуск оттуда же после core:
  `HTTP_PORT=8088 SYSTEM_PORT=3004 CORE_GRPC_ADDRESS=localhost:5050 SDK_JS_FILE=<repo>/sdk/js/dist/ruto-auth.iife.js ./gateway` → трафик на `http://localhost:8088/<path_prefix>/...`.
  - `SDK_JS_FILE` указывает на сборку JS SDK (перед этим `make build-sdk`), шлюз отдаёт её на `http://localhost:8088/_auth/ruto-auth.js`; без файла маршрут отвечает 404 (предупреждение в логе при старте).
  - Бэкенд-заглушка для приложений стенда: любой локальный http-сервер на `http://127.0.0.1:18081`.
  - Отчёт о замеченных адресах шлюз шлёт в core раз в 30 с — после запросов ждать до полуминуты.
- **admin (vite):** `VITE_API_BASE_URL=http://localhost:9090/api VITE_HOST=localhost VITE_PORT=5173 pnpm --dir apps/admin dev` → `http://localhost:5173`
- **Первый администратор** (БД пустая — создаётся на экране логина): name `Test Admin`, username `admin`, password `ruto-test-admin`.
- **Keycloak (для проверок SSO: вход, `me`, refresh, logout):** `docker run --rm -d --name ruto-test-kc -p 8180:8080 -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin -v <repo>/config/test-stand/keycloak:/opt/keycloak/data/import:ro quay.io/keycloak/keycloak:26.4 start-dev --import-realm --hostname http://localhost:8180 --http-port 8080` (стартует ~30 с, готовность — `GET http://localhost:8180/realms/corp/.well-known/openid-configuration`).
  - Realm `corp` из `config/test-stand/keycloak/corp-realm.json`: issuer `http://localhost:8180/realms/corp`, access-токены (общий и токены приложений) живут 5 мин — у Keycloak срок токена обмена равен сроку access-токена клиента шлюза, отдельно не задать; сессия (refresh) — 7 дней. Клиент шлюза `ruto` / секрет `ruto-secret` (callback `http://localhost:8088/_auth/corp/callback`, mapper своей claim `tenant_id`, mapper «Audience» с `ruto`, включён Standard token exchange, область `roles` — необязательная), клиенты-контейнеры ролей `shop` и `orders` с ролью `manager`, `crm` и `cart-fe` с ролью `user` (это клиенты фронтов брокера: для Keycloak — только аудитории обмена, вход у них выключен), realm-роль `auditor`. Включён журнал событий обмена: `GET /admin/realms/corp/events?type=TOKEN_EXCHANGE&type=TOKEN_EXCHANGE_ERROR` (токен админа master `admin`/`admin`) — так считаются обмены.
  - Пользователи: `alice` / `alice-pass` (роли `auditor`, `shop:manager`, `orders:manager`, `crm:user`, `cart-fe:user`, `tenant_id=t-42`), `bob` / `bob-pass` (без ролей — для проверки 403 и входа во фронт без ролей).
  - Конфиг core под этот стенд одним скриптом: `python3 -I config/test-stand/configure.py` (админ, провайдер `corp` — вход без `roles`, `exchange_scopes: [roles]`, CORS для фронтов, приложения `shop` — без своей аудитории, бэкенд получает общий токен, и `orders`, `orders-api` — аудитория `orders`, роль `orders:manager`, `crm-api` — аудитория `crm` (клиент фронта crm), роль `crm:user`, по endpoint'у `GET hello`, публикация). Бэкенд-заглушка — `python3 -I config/test-stand/backend.py` (`127.0.0.1:18081`, отдаёт путь, начало заголовка Authorization, куки и `token` — содержимое полученного токена без проверки).
  - Тестовые фронты — статические страницы `config/test-stand/fronts/{shop,orders}/index.html`: `python3 -I -m http.server 5181 --bind 127.0.0.1` в `fronts/shop` и `5182` в `fronts/orders` → `http://localhost:5181/`, `http://localhost:5182/`. Страницы работают на JS SDK `sdk/js` (`@rendau/ruto-auth`): браузерную сборку они берут со шлюза, `http://localhost:8088/_auth/ruto-auth.js`, поэтому шлюз запускать с `SDK_JS_FILE` (см. выше) после `make build-sdk`. На странице кнопки Login / Me / Call API / Refresh / Logout (у orders — ещё Call orders-api; обе страницы на общем входе); состояние доступно из JS: `window.__me`, `window.__api` (у orders — с `token`, токеном, который получил бэкенд), `window.__loginRequired` (сколько раз сработал `onLoginRequired`), `window.__ruto.{me,call,refresh,login,logout}()` и сам клиент `window.__ruto.auth`.
  - Фронты брокера (вход «в ruto» стандартными SDK): `config/test-stand/fronts/crm/index.html` на keycloak-js (клиент `crm`, `checkLoginIframe: false`, `check-sso` при открытии) и `fronts/cart/index.html` на oidc-client-ts (клиент `cart-fe`, `fetchRequestCredentials: 'include'`; на нём построен auth-js). Библиотеки — из npm: один раз `pnpm --dir config/test-stand/fronts install` (`node_modules` не в git). Раздаются из корня `fronts`: `python3 -I -m http.server 5183 --bind 127.0.0.1 -d config/test-stand/fronts` и то же на `5184` → `http://localhost:5183/crm/`, `http://localhost:5184/cart/`. Состояние из JS: у crm — `window.__kc`, `window.__state.summary()`, `window.__state.api` (ответ `crm-api`, с `token`, который получил бэкенд); у cart — `window.__oidc`, `window.__state.summary()`.
  - Повтор кода брокера отсекается только с Redis: `docker run --rm -d --name ruto-test-redis -p 56379:6379 -m 128m redis:7-alpine redis-server --maxmemory 64mb --maxmemory-policy allkeys-lru` и core с `GW_REDIS_ADDR=localhost:56379`.
- После проверок: остановить vite, core, gateway, заглушку и фронты (core и шлюз запущены как `./core` и `./gateway` — искать их `pgrep -f '^\./(core|gateway)$'`, по полному пути `pkill` их не найдёт), `docker stop ruto-test-pg ruto-test-kc` (и `ruto-test-redis`, если поднимали).
