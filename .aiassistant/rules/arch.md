---
apply: manual
---

# Architecture

- `core`: config -> pgx -> migrations -> repos/services/usecases -> gRPC handlers.
- `core` serves gRPC, grpc-gateway `/api/*`, and SPA `/` when dist exists.
- `gateway`: connects to `core`, loads snapshot, serves HTTP/gRPC proxy and system endpoints.
- Gateway gRPC routing uses metadata `x-ruto-app-name` plus method path.
- Cache: Redis if `REDIS_ADDR` is set, otherwise memory.
- Swagger backfill: `APP_SWAGGER_DISCOVERY_ON_START=true`.

## Landmarks

- Wiring: `internal/app/{core,gateway}`
- Config: `internal/config/{core,gateway}`
- Domain/usecases/transport: `internal/domain/*`, `internal/usecase/*`, `internal/handler/grpc/*`
- Gateway internals: `internal/service/gw/*`
- Swagger: `internal/service/swagger/*`
- DB: `migrations/*.sql`

## Workflows

- API: proto -> `make generate-proto` -> handlers/usecases/tests.
- DB: migration -> repo models/queries -> impacted tests.
