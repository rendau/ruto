---
apply: manual
---

# Runbook

- Setup env: copy `config/core.env.example` -> `.env.core`; `config/gateway.env.example` -> `.env.gateway`.
- Run: `make run-core`, `make run-gateway`, optional `make run-admin`.
- Build: `make build-all`, `make build-admin`, `make build-prod`.
- Test all: `go test ./...`.
- Focused tests: `go test ./internal/usecase/app/...`, `go test ./internal/service/gw/...`, `go test ./internal/domain/...`.
- Proto: edit `api/proto/ruto_v1/*.proto`; run `make generate-proto`.
- Migration: `migrate create -ext sql -dir migrations <name>`.
