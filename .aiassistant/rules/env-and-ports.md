---
apply: always
---

# Environment and Ports

## Core (`.env.core`)

- required:
  - `ADMIN_JWT_SECRET`
  - `PG_DSN`
- key defaults:
  - `SYSTEM_PORT=3003`
  - `GRPC_PORT=5050`
  - `HTTP_PORT=80`
  - `HTTP_CORS=false`
- optional:
  - Redis: `REDIS_ADDR`, `REDIS_DB`, `REDIS_PASSWORD`
  - legacy migration: `LEGACY_DM_BASE_URL`, `LEGACY_DM_REFRESH_TOKEN`
  - startup backfill: `APP_SWAGGER_DISCOVERY_ON_START=false`

## Gateway (`.env.gateway`)

- required:
  - `HTTP_PORT`
  - `GRPC_PORT`
  - `CORE_GRPC_ADDRESS`
- key defaults:
  - `SYSTEM_PORT=3003`
  - `LOG_REQUESTS=false`
- optional:
  - `TRUSTED_PROXY_ADDRESSES` (comma-separated)

## Common local expectations

- Core gRPC target for gateway usually: `CORE_GRPC_ADDRESS=localhost:5050`
- Admin SPA default API base points to core HTTP host (`http://localhost:80` unless overridden in admin env)
