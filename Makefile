.DEFAULT_GOAL := build-all

CORE_BINARY_NAME = core
GATEWAY_BINARY_NAME = gateway
BUILD_PATH = cmd/build
SERVICE_NAME = ruto_v1
ADMIN_PATH = apps/admin
SDK_JS_PATH = sdk/js

# Тулчейн генерации proto — локально в проекте (bin/, в .gitignore), версии закреплены:
# у всех разработчиков генерация даёт одинаковый вывод. protoc-gen-go и grpc-gateway берутся
# версий рантайм-библиотек из go.mod — сгенерированный код совпадает с тем, с чем собирается сервис.
LOCAL_BIN := $(CURDIR)/bin
PROTOC_VERSION := 36.2
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
PROTOC_GEN_GO_VERSION = $(shell go list -m -f '{{.Version}}' google.golang.org/protobuf)
GRPC_GATEWAY_VERSION = $(shell go list -m -f '{{.Version}}' github.com/grpc-ecosystem/grpc-gateway/v2)

PROTOC_OS := $(shell uname -s | sed -e 's/Darwin/osx/' -e 's/Linux/linux/')
PROTOC_ARCH := $(shell uname -m | sed -e 's/arm64/aarch_64/' -e 's/aarch64/aarch_64/')
PROTOC_URL = https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip
PROTOC := $(LOCAL_BIN)/protoc

# плагины указываются явно: отсутствующий локальный плагин — ошибка, а не молчаливый глобальный из PATH
PROTOC_PLUGINS = \
	--plugin=protoc-gen-go=$(LOCAL_BIN)/protoc-gen-go \
	--plugin=protoc-gen-go-grpc=$(LOCAL_BIN)/protoc-gen-go-grpc \
	--plugin=protoc-gen-grpc-gateway=$(LOCAL_BIN)/protoc-gen-grpc-gateway \
	--plugin=protoc-gen-openapiv2=$(LOCAL_BIN)/protoc-gen-openapiv2

.SILENT:

run-core:
	go run cmd/core/main.go

run-gateway:
	go run cmd/gateway/main.go

run-admin:
	pnpm --dir $(ADMIN_PATH) dev

build-core:
	mkdir -p $(BUILD_PATH)
	CGO_ENABLED=0 go build -o $(BUILD_PATH)/$(CORE_BINARY_NAME) cmd/core/main.go

build-gateway:
	mkdir -p $(BUILD_PATH)
	CGO_ENABLED=0 go build -o $(BUILD_PATH)/$(GATEWAY_BINARY_NAME) cmd/gateway/main.go

build-all: build-core build-gateway

build-admin:
	pnpm --dir $(ADMIN_PATH) install
	pnpm --dir $(ADMIN_PATH) build

build-prod: build-all build-admin

# JS SDK входа через шлюз (sdk/js): dist/ нужен тестовым фронтам стенда (config/test-stand/fronts)
build-sdk:
	pnpm --dir $(SDK_JS_PATH) install
	pnpm --dir $(SDK_JS_PATH) build

test-sdk:
	pnpm --dir $(SDK_JS_PATH) install
	pnpm --dir $(SDK_JS_PATH) test

clean:
	rm -rf $(BUILD_PATH)

# Установка тулчейна генерации proto в bin/ — в итоге там только бинарники: protoc и плагины.
# protoc: из релиза GitHub берётся один bin/protoc (архив — во временном каталоге, удаляется);
# скачивается, только если в bin/ нет protoc нужной версии. include/ релиза не нужен —
# стандартные google/protobuf/*.proto лежат в vendor-proto.
# Плагины: go install с закреплёнными версиями (из кэша модулей — без сети).
proto-tools:
	mkdir -p $(LOCAL_BIN)
	if [ "$$($(PROTOC) --version 2>/dev/null)" != "libprotoc $(PROTOC_VERSION)" ]; then \
		tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
		curl -fsSL -o "$$tmp/protoc.zip" $(PROTOC_URL) && \
		unzip -qjo "$$tmp/protoc.zip" bin/protoc -d $(LOCAL_BIN); \
	fi
	GOBIN=$(LOCAL_BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(LOCAL_BIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	GOBIN=$(LOCAL_BIN) go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@$(GRPC_GATEWAY_VERSION)
	GOBIN=$(LOCAL_BIN) go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(GRPC_GATEWAY_VERSION)

proto-tools-version: proto-tools
	$(PROTOC) --version
	for p in protoc-gen-go protoc-gen-go-grpc protoc-gen-grpc-gateway protoc-gen-openapiv2; do \
		printf '%-24s %s\n' $$p "$$(go version -m $(LOCAL_BIN)/$$p | awk '$$1=="mod"{print $$3}')"; \
	done

generate-proto-$(SERVICE_NAME):
	mkdir -p pkg/proto
	$(PROTOC) $(PROTOC_PLUGINS) -I vendor-proto -I api/proto \
	--go_out pkg/proto --go_opt paths=source_relative \
	--go_opt=Mcommon/common.proto=`go list -m `/pkg/proto/common \
	--go-grpc_out pkg/proto --go-grpc_opt paths=source_relative \
	--grpc-gateway_out pkg/proto --grpc-gateway_opt paths=source_relative \
	--openapiv2_out=json_names_for_fields=false,allow_merge=true,merge_file_name=api:docs \
	api/proto/$(SERVICE_NAME)/*.proto

generate-proto: proto-tools generate-proto-$(SERVICE_NAME)
