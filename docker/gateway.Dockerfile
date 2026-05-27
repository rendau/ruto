FROM golang:1.26-alpine AS gateway-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -o /out/gateway ./cmd/gateway/main.go

FROM alpine:latest

RUN apk add --no-cache --upgrade ca-certificates tzdata curl

WORKDIR /app

COPY --from=gateway-builder /out/gateway ./gateway

CMD ["./gateway"]
