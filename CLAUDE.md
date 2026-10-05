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
- `docs/code-style.md` — conventions.
- `docs/onboarding.md` — local setup and recipes for common changes.
- `docs/gateway-grpc.md` — connecting to the gateway over gRPC.
- `docs/monitoring.md` — admin traffic charts & logs (Prometheus / Loki / Graylog integration).

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
  - HTTP `:9090` (REST `/api/*`), gRPC `:5050`, system `:3003`.
- **gateway (нужен только для проверок трафика):** `go build -o <scratchpad>/stand/gateway cmd/gateway/main.go`, запуск оттуда же после core:
  `HTTP_PORT=8088 SYSTEM_PORT=3004 CORE_GRPC_ADDRESS=localhost:5050 ./gateway` → трафик на `http://localhost:8088/<path_prefix>/...`.
  - Бэкенд-заглушка для приложений стенда: любой локальный http-сервер на `http://127.0.0.1:18081`.
  - Отчёт о замеченных адресах шлюз шлёт в core раз в 30 с — после запросов ждать до полуминуты.
- **admin (vite):** `VITE_API_BASE_URL=http://localhost:9090/api VITE_HOST=localhost VITE_PORT=5173 pnpm --dir apps/admin dev` → `http://localhost:5173`
- **Первый администратор** (БД пустая — создаётся на экране логина): name `Test Admin`, username `admin`, password `ruto-test-admin`.
- После проверок: остановить vite и core, `docker stop ruto-test-pg`.
