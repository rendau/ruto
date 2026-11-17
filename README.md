# ruto

Управляемый API-gateway: `core` (control plane) хранит конфигурацию и публикует snapshot, `gateway` (data plane) проксирует трафик на backend'ы по этому snapshot.

📚 **Документация для погружения — в [`docs/`](docs/README.md):**
[обзор](docs/overview.md) · [архитектура](docs/architecture.md) · [стиль кода](docs/code-style.md) · [онбординг](docs/onboarding.md) · [gateway gRPC](docs/gateway-grpc.md).

## Backends

Проект разделен на два бэкенда:

- `core`: БД + API + snapshot source
- `gateway`: тянет snapshot из `core` и поднимает gateway

Entrypoints:

- `cmd/core/main.go`
- `cmd/gateway/main.go`

Команды:

```bash
make run-core
make run-gateway
make build-all
```

Env templates:

- `config/core.env.example`
- `config/gateway.env.example`

Gateway gRPC clients:

- `docs/gateway-grpc.md`

### DB dump:

```
pg_dump --no-owner -Fc -U postgres ruto -f ./ruto.custom
```

### DB restore:

```
dropdb -U postgres ruto
createdb -U postgres ruto
pg_restore --no-owner -d ruto -U postgres ./ruto.custom
```

### Install `migrate` command-tool:

https://github.com/golang-migrate/migrate/tree/master/cmd/migrate

### Create new migration:

```
migrate create -ext sql -dir migrations mg_name
```

### Apply migration:

```
migrate -path migrations -database "postgres://localhost:5432/db_name?sslmode=disable" up
```
