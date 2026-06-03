---
apply: always
---

# Architecture Map (ruto)

## High-level flow

1. `core` starts, loads config, initializes pgx pool, runs migrations.
2. `core` wires domain repos/services/usecases and gRPC handlers.
3. `core` serves:
   - gRPC on `GRPC_PORT`
   - grpc-gateway HTTP on `HTTP_PORT` (`/api/*`)
   - admin SPA on `/` (if frontend dist exists)
4. `gateway` starts separately, connects to `core` using `CORE_GRPC_ADDRESS`.
5. `gateway` exposes:
   - proxied HTTP and gRPC listeners
   - system server (`SYSTEM_PORT`) with `/healthcheck` and optional Prometheus metrics

## Source tree landmarks

- `cmd/` executable entrypoints
- `internal/app/core` service wiring for core
- `internal/app/gateway` service wiring for gateway
- `internal/config/{core,gateway}` env parsing and config defaults
- `internal/domain/*` domain models/repos/services
- `internal/usecase/*` business orchestration layer
- `internal/handler/grpc/*` inbound gRPC transport
- `internal/service/gw/*` gateway runtime/proxy/auth/metrics internals
- `internal/service/swagger/*` swagger parsing/discovery logic
- `migrations/*.sql` DB migrations
- `api/proto/ruto_v1/*.proto` proto source of truth
- `pkg/proto/ruto_v1/*` generated Go/grpc-gateway stubs

## Operational notes

- `core` can use Redis cache when `REDIS_ADDR` is set; otherwise in-memory cache.
- Swagger URL backfill can run on startup when `APP_SWAGGER_DISCOVERY_ON_START=true`.
- gRPC routing in gateway depends on metadata header `x-ruto-app-name` and method path.

## Change guidance

- For API contract changes:
  1. update proto in `api/proto/ruto_v1`
  2. run `make generate-proto`
  3. update handlers/usecases/tests
- For DB changes:
  1. add migration in `migrations/`
  2. update domain repo model/query code
  3. validate impacted usecases/tests
