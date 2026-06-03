---
apply: always
---

# Project Context (ruto)

## Stack and layout

- Monorepo with Go backend services and Vue 3 admin SPA.
- Main backend entrypoints:
  - `cmd/core/main.go`
  - `cmd/gateway/main.go`
- Admin SPA:
  - `apps/admin` (Vite + Vue 3 + Pinia + Vue Router + TypeScript)

## Services

- `core`:
  - owns Postgres-backed domain data
  - applies DB migrations on startup
  - exposes gRPC API
  - exposes HTTP API via grpc-gateway under `/api/*`
  - serves admin SPA from `./admin-dist` or `./apps/admin/dist` when available
- `gateway`:
  - gets routing snapshot from `core`
  - serves HTTP + gRPC proxy endpoints for downstream apps
  - has system endpoints (`/healthcheck`, optional `/metrics`)

## Primary working commands

- `make run-core`
- `make run-gateway`
- `make run-admin`
- `make build-all`
- `make build-admin`
- `make generate-proto`
- tests: `go test ./...`

## Environment files

- `core` reads `.env.core` (`config/core.env.example`)
- `gateway` reads `.env.gateway` (`config/gateway.env.example`)

## Edit policy for this repo

- Prefer editing source proto files in `api/proto/ruto_v1/*.proto`, then regenerate to `pkg/proto/ruto_v1/*`.
- Treat `pkg/proto/ruto_v1/*` as generated artifacts.
- Keep existing layered structure:
  - `internal/domain/*`
  - `internal/usecase/*`
  - `internal/handler/grpc/*`
  - infra/services under `internal/service/*`
- Keep changes scoped; avoid unrelated refactors.
