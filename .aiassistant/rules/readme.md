---
apply: always
---

# Ruto Context

Go monorepo: `core`, `gateway`, Vue 3 admin SPA in `apps/admin`.

## Map

- Entrypoints: `cmd/core/main.go`, `cmd/gateway/main.go`.
- Layers: `internal/domain/*`, `internal/usecase/*`, `internal/handler/grpc/*`, `internal/service/*`.
- Proto source: `api/proto/ruto_v1/*.proto`; generated: `pkg/proto/ruto_v1/*`.
- Env templates: `config/core.env.example`, `config/gateway.env.example`.

## Rules

- Keep changes scoped; avoid unrelated refactors.
- Edit proto source before generated files; run `make generate-proto`.
- Treat `pkg/proto/ruto_v1/*` as generated.
- Use `.aiassistant/rules/preferences.md`.
- core — источник конфигурации snapshot и бизнес-логики, так же бэкенд для apps/admin
- gateway работает как proxy

## Commands

`make run-core`, `make run-gateway`, `make run-admin`, `make build-all`, `make build-admin`, `go test ./...`.

Load detail only when relevant: `arch.md`, `env-and-ports.md`, `runbook.md`.
