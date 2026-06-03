---
apply: always
---

# Dev Runbook

## Local startup (typical)

1. Prepare env files:
   - copy `config/core.env.example` -> `.env.core`
   - copy `config/gateway.env.example` -> `.env.gateway`
2. Start core: `make run-core`
3. Start gateway: `make run-gateway`
4. (optional) Start admin UI in dev mode: `make run-admin`

## Build targets

- backend binaries: `make build-all`
- admin build: `make build-admin`
- full build: `make build-prod`

## Tests and checks

- all Go tests: `go test ./...`
- focused tests:
  - `go test ./internal/usecase/app/...`
  - `go test ./internal/service/gw/...`
  - `go test ./internal/domain/...`

## API/proto workflow

- edit proto sources in `api/proto/ruto_v1/*.proto`
- regenerate stubs/gateway/swagger: `make generate-proto`

## DB workflow

- create migration:
  - `migrate create -ext sql -dir migrations <name>`
- apply migration:
  - `migrate -path migrations -database "postgres://localhost:5432/<db>?sslmode=disable" up`
