# Соглашения по коду

Свод устоявшихся в репозитории паттернов. Следуй им, чтобы новый код выглядел единообразно.

## Общие правила Go

- **Go 1.26+**: для указателей на литералы используй `new(value)` вместо временной переменной ради `&tmp`. Пример: `obj.Active = new(true)`, `pars.Page = new(int32(1))`.
- **Все методы структур — pointer-методы** (`func (m *T) ...`), даже read-only/чистые: единообразие приёмников, отсутствие неявных копий и возможность вызывать на адресуемых значениях. Не смешивай value- и pointer-приёмники у одного типа.
- Активно используй **`samber/lo`** там, где это уместно: `lo.Map`, `lo.Filter`, `lo.FilterMap`, `lo.ForEach`.
- Не добавляй избыточные `strings.TrimSpace` и nil-проверки для уже нормализованных доменных сущностей — их инварианты гарантируют валидность. Нормализация делается один раз, в `Normalize()`/`validate*`.
- Изменения держи **локальными**: не делай несвязанных рефакторингов в рамках задачи.
- Комментарии к функциям/блокам кода пиши **только при логической необходимости** — когда они объясняют неочевидное «почему» (инвариант, неочевидный нюанс, причина решения). Не комментируй самоочевидный код и не дублируй словами то, что и так читается из кода.

## Конструкторы

Конструктор всегда `New(...)` и возвращает **указатель**. Зависимости передаются позиционными аргументами (без options-структур для простых случаев):

```go
func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }
```

## Интерфейсы объявляются у потребителя

Каждый слой описывает нужные ему зависимости как интерфейс **в своём пакете**, в файле `interfaces.go`. Реализация про эти интерфейсы не знает.

- usecase → `ServiceI`, `SessionServiceI`, … (то, что он вызывает у domain).
- domain/service → `RepoDbI` (то, что он вызывает у repo).

Имена интерфейсов — с суффиксом `I` (`ServiceI`, `RepoDbI`, `RootServiceI`).

## Обработка ошибок

- Оборачивай ошибку именем вызванной функции через `%w`:
  ```go
  if err != nil {
      return nil, fmt.Errorf("repoDb.Get: %w", err)
  }
  ```
- Доменные ошибки — sentinel-значения из `internal/errs` (тип `errs.Err string`). Возвращай их **без обёртки**, чтобы транспорт мог их распознать: `return errs.NotAuthorized`.
- Для ошибок с деталями (описание, поля формы) — `errs.ErrFull{Err, Desc, Fields}`.
- Распознавание на границе — через `errors.AsType[errs.Err]` / `errors.AsType[errs.ErrFull]`.
- Паттерн «найдено/не найдено»: сервисы возвращают `(*T, bool, error)`; флаг `errNE bool` решает, превращать ли «не найдено» в `errs.ObjectNotFound`.

## Слой transport (handler/grpc)

- Хендлер встраивает `ruto_v1.Unsafe<Svc>Server` и держит указатель на usecase.
- Тело метода: декод proto → вызов usecase → энкод proto. Ошибку возвращай как есть.
- Маппинг proto ↔ domain вынесен в `internal/handler/grpc/dto/*` функциями `Decode<X>` / `Encode<X>`. `Encode*` имеют сигнатуру под `lo.Map` (`func(item *T, _ int) *protoT`).

```go
func (h *Usr) Get(ctx context.Context, req *ruto_v1.UsrGetReq) (*ruto_v1.UsrMain, error) {
    item, err := h.usecase.Get(ctx, req.Id)
    if err != nil {
        return nil, err
    }
    return dto.EncodeUsrMain(item, 0), nil
}
```

## Слой usecase

- Первым делом — проверка доступа по session: `u.sessionSvc.CtxIsAuthorized(ctx)` / `CtxIsAdmin(ctx)`, иначе `errs.NotAuthorized` / `errs.NoPermission`.
- Валидация входа — приватные методы `validate*`; нормализация строк (`TrimSpace`) — здесь, до записи.
- Session берётся из контекста (`u.sessionSvc.FromContext(ctx)`), а не из аргументов.

## Слой repo (db)

- `Repo` встраивает `*commonRepoPg.Base` (даёт `Con` и `QB`) и держит `*mobone.ModelStore` с `TableName`.
- Отдельные **repo-модели** под `repo/db/model/` (`Select`, `Upsert`, `GetByUsername`, …) — отдельно от domain-моделей. Конвертация — `EncodeSelect` (repo→domain), `DecodeUpsertEdit` (domain→repo).
- Списки — через `ModelStore.List` + `mobone.ListParams`; сортировка — `moboneTools.ConstructSortColumns(allowedSortFields, …)`.
- Кастомные запросы — в `custom.go`, query-builder `squirrel` с плейсхолдерами `$N` (`PlaceholderFormat(squirrel.Dollar)`).

## Доменные модели и преобразования

Модели сущностей конфигурации имеют набор чистых методов-преобразований, разнесённых по файлам:

| Файл | Метод | Назначение |
| --- | --- | --- |
| `normalize.go` | `Normalize()` | привести к каноничному/валидному виду |
| `inherit.go` | `InheritDown()` | спустить auth/vars родителя в детей |
| `interpolate.go` | `Interpolate()` | подставить переменные в строки |
| `merge.go` | `Merge(parent, child)` | слить auth с учётом `extend`/`replace` |

Поля сериализуются в jsonb — у всех полей теги `json:"snake_case"`. Поля, не хранимые в БД (вложенные дети, распарсенные URL), помечены комментарием `// not stored in db` или тегом `json:"-"`.

## Именование импортов

- В **composition root** (`internal/app/*`) пакеты алиасятся с суффиксом `P`, чтобы развести одноимённые пакеты: `domainUsrRepoDbP`, `usecaseAppP`, `handlerGrpcP`.
- В обычных файлах — короткие осмысленные алиасы при коллизиях: `domainModel`, `repoModel`, `sessionModel`, `authModel`.

## Тесты

- `stretchr/testify`: `assert` для мягких проверок, `require` — где дальше нет смысла продолжать.
- Тесты лежат рядом с кодом (`*_test.go`), особенно плотно покрыты чистые преобразования моделей (`interpolate_test.go`, `merge_test.go`, `normalize`-логика) и auth-методы.
- Запуск: `go test ./...`; точечно — `go test ./internal/usecase/app/...`.

## Логирование

- Стандартный `log/slog` со структурными полями: `slog.Error("...", "error", err)`, `slog.Info(...)`.
- Инициализация уровня/режима — `internal/app/common/logger.go` (`InitLogger(debug, level)`).

## Proto / генерация

- Редактируй **только** исходники `api/proto/ruto_v1/*.proto`, затем `make generate-proto`.
- `pkg/proto/ruto_v1/*` — сгенерированный код, руками не трогать.
