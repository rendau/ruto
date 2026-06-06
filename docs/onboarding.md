# Онбординг и рецепты

Практическое руководство: как запустить проект локально и как вносить типовые изменения.

## Локальный запуск

1. **Подготовь env-файлы** (шаблоны в `config/`):
   ```bash
   cp config/core.env.example .env.core
   cp config/gateway.env.example .env.gateway
   ```
   Для `core` минимально нужны `ADMIN_JWT_SECRET` и `PG_DSN`. Для `gateway` — `HTTP_PORT`, `GRPC_PORT`, `CORE_GRPC_ADDRESS` (локально обычно `localhost:5050`).

2. **Подними PostgreSQL** и пропиши `PG_DSN` в `.env.core`. Миграции `core` накатывает сам при старте.

3. **Запусти бэкенды** (каждый в своём терминале):
   ```bash
   make run-core      # control plane: gRPC :5050, HTTP :80 (REST /api/* + SPA)
   make run-gateway   # data plane: proxy на портах из .env.gateway
   ```

4. **Админ-SPA** (опционально, для разработки фронта):
   ```bash
   make run-admin     # vite dev-сервер, по умолчанию проксирует на http://localhost:80
   ```

### Порты по умолчанию

| Переменная | core | gateway |
| --- | --- | --- |
| `SYSTEM_PORT` | 3003 (healthcheck/metrics) | 3003 |
| `GRPC_PORT` | 5050 | задаётся вручную |
| `HTTP_PORT` | 80 | задаётся вручную |

Кэш: если задан `REDIS_ADDR` — Redis, иначе in-memory.

## Сборка и тесты

```bash
make build-all      # бинарники core и gateway → cmd/build/
make build-admin    # сборка SPA
make build-prod     # всё вместе
go test ./...       # все тесты
```

## Рецепт: добавить новый gRPC/REST метод в core

1. Опиши rpc и сообщения в `api/proto/ruto_v1/<service>.proto` (HTTP-биндинг через `option (google.api.http)`).
2. Сгенерируй код: `make generate-proto`.
3. Добавь `Decode*`/`Encode*` в `internal/handler/grpc/dto/<service>.go`.
4. Реализуй метод хендлера в `internal/handler/grpc/<service>.go` (тонкий: decode → usecase → encode).
5. Добавь метод в usecase (`internal/usecase/<service>`) — там авторизация и логика; при необходимости расширь его `interfaces.go`.
6. Если нужен новый вызов к БД — спускай вниз: domain/service → его `RepoDbI` → repo/db.
7. Покрой тестами (usecase/domain). Прогон: `go test ./...`.

> Хендлер уже зарегистрирован в `internal/app/core/app.go` (блок `RegisterXxxServer` и список `RegisterXxxHandler`). Новый **сервис** добавляется туда; новый **метод** существующего сервиса — нет.

## Рецепт: добавить новую сущность (полный вертикальный срез)

Повтори структуру существующей сущности (`usr` — хороший образец):

```
internal/domain/<e>/
  model/        model.go (+ normalize/inherit/interpolate при необходимости)
  service/      service.go, interfaces.go (RepoDbI)
  repo/db/      repo.go, custom.go, model/{select.go,upsert.go}
internal/usecase/<e>/   usecase.go, interfaces.go, model.go
internal/handler/grpc/  <e>.go + dto/<e>.go
api/proto/ruto_v1/<e>.proto
migrations/             *_<e>.up.sql / *.down.sql
```

Затем свяжи всё в `internal/app/core/app.go` (repo → service → usecase → handler → `Register…`).

## Рецепт: миграция БД

```bash
migrate create -ext sql -dir migrations <name>   # создаст up/down пару
```
Запиши SQL в `*.up.sql` (и откат в `*.down.sql`). `core` применяет миграции автоматически при старте (`internal/app/core/migration.go`). Конфигурационные сущности (`app`, `endpoint`, `root`) хранят модель в `data jsonb` — для изменения их структуры миграция схемы обычно **не нужна**, достаточно поправить Go-модель.

## Рецепт: новый метод авторизации в gateway

1. Добавь вариант в `internal/domain/auth/model/model.go` (`AuthMethodType`, поле в `AuthMethod`).
2. Реализуй authorizer в `internal/service/gw/service/auth/authorizer/<method>/`.
3. Подключи его в `internal/service/gw/service/auth/method.go`.
4. Тесты рядом (см. `authorizer/jwt`, `authorizer/ip_validation`).

## Рецепт: подключиться к gateway по gRPC

См. отдельный документ [gateway-grpc.md](gateway-grpc.md): какие metadata-заголовки слать (`x-ruto-app-name`), как настроить App/Endpoint, примеры `grpcurl` и Go-клиента, разбор частых ошибок.

## Где что искать

| Нужно… | Смотри |
| --- | --- |
| как собираются зависимости | `internal/app/core/app.go`, `internal/app/gateway/app.go` |
| как читается конфиг/env | `internal/config/{core,gateway}/config.go` |
| как ошибка превращается в gRPC-ответ | `internal/app/core/grpc.go` (`GrpcInterceptorError`) |
| как gateway получает конфиг | `internal/service/gw/core_client/service.go` |
| как строятся маршруты прокси | `internal/service/gw/handler/http/service.go` |
| как собирается snapshot | `internal/usecase/snapshot/usecase.go` |
| модель конфигурации | `internal/domain/{root,app,endpoint}/model` |
| контракт API | `api/proto/ruto_v1/*.proto` |

Концепции и слои — в [architecture.md](architecture.md), стиль — в [code-style.md](code-style.md).
