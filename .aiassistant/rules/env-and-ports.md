---
apply: manual
---

# Env And Ports

- Core env: `.env.core` from `config/core.env.example`.
- Core required: `ADMIN_JWT_SECRET`, `PG_DSN`.
- Core defaults: `SYSTEM_PORT=3003`, `GRPC_PORT=5050`, `HTTP_PORT=80`, `HTTP_CORS=false`.
- Core optional: Redis vars, legacy migration vars, `APP_SWAGGER_DISCOVERY_ON_START=false`.
- Gateway env: `.env.gateway` from `config/gateway.env.example`.
- Gateway required: `HTTP_PORT`, `GRPC_PORT`, `CORE_GRPC_ADDRESS`.
- Gateway defaults: `SYSTEM_PORT=3003`, `LOG_REQUESTS=false`.
- Local gateway target usually: `CORE_GRPC_ADDRESS=localhost:5050`.
