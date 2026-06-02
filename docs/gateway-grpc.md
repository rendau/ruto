# Подключение к gateway по gRPC

Gateway поднимает отдельный gRPC listener и проксирует вызовы в backend по snapshot из `core`.

## Адрес gateway

Локально gateway читает настройки из `.env.gateway`:

```env
GRPC_PORT=3031
CORE_GRPC_ADDRESS=localhost:5050
```

Для клиента это означает:

```text
localhost:3031
```

`CORE_GRPC_ADDRESS` нужен самому gateway для получения snapshot из `core`; внешние клиенты к нему не подключаются.

## Как gateway выбирает приложение

Для каждого gRPC-вызова клиент должен передать metadata header:

```text
x-ruto-app-name: <app name>
```

`<app name>` должен совпадать с `Name` приложения в админке.

Gateway ищет маршрут по двум значениям:

- `x-ruto-app-name`
- полный gRPC method path, например `/package.Service/Method`

Если header не передан или endpoint не зарегистрирован, gateway вернет `NotFound: route not found`.

## Что должно быть настроено в админке

В приложении:

- `Name`: имя, которое клиент передает в `x-ruto-app-name`.
- `Backend URL`: HTTP(S) URL backend-хоста. Для gRPC gateway использует hostname из этого URL.
- `gRPC Port`: порт backend gRPC сервера.

В endpoint:

- `Type`: `gRPC`.
- `Service`: например `package.Service`.
- `Method`: например `Method`.
- `gRPC Path`: например `/package.Service/Method`.
- `Active`: enabled.

Gateway проксирует вызов на:

```text
<hostname из Backend URL>:<gRPC Port>
```

Например, если `Backend URL = https://api.internal` и `gRPC Port = 50051`, target будет:

```text
api.internal:50051
```

## Пример grpcurl

```bash
grpcurl \
  -plaintext \
  -H 'x-ruto-app-name: account' \
  -d '{"id":"123"}' \
  localhost:3031 \
  package.Service/Method
```

Посмотреть services, зарегистрированные в gateway для конкретного app:

```bash
grpcurl \
  -plaintext \
  -H 'x-ruto-app-name: account' \
  localhost:3031 \
  list
```

Если endpoint защищен auth, передайте нужные metadata headers дополнительно:

```bash
grpcurl \
  -plaintext \
  -H 'x-ruto-app-name: account' \
  -H 'x-api-key: secret' \
  -d '{"id":"123"}' \
  localhost:3031 \
  package.Service/Method
```

## Пример Go client

```go
conn, err := grpc.NewClient(
	"localhost:3031",
	grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
	return err
}
defer conn.Close()

ctx := metadata.AppendToOutgoingContext(
	context.Background(),
	"x-ruto-app-name", "account",
)

client := pb.NewServiceClient(conn)
resp, err := client.Method(ctx, &pb.Request{Id: "123"})
if err != nil {
	return err
}
_ = resp
```

## Частые ошибки

- `route not found`: нет `x-ruto-app-name`, app name не совпадает, endpoint inactive, app inactive или `gRPC Path` не совпадает с реальным `/package.Service/Method`.
- `Unauthenticated`: endpoint/app/root auth включен, но клиент не передал нужные metadata headers.
- `Unavailable: dial backend`: gateway нашел route, но не смог подключиться к `<Backend hostname>:<gRPC Port>`.
- HTTP gateway port не подходит для gRPC. Для gRPC нужен `GRPC_PORT`, сейчас локально это `3031`.
