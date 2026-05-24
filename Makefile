.DEFAULT_GOAL := build-all

CORE_BINARY_NAME = core
GATEWAY_BINARY_NAME = gateway
BUILD_PATH = cmd/build
SERVICE_NAME = ruto_v1
ADMIN_PATH = apps/admin

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

clean:
	rm -rf $(BUILD_PATH)

generate-proto-$(SERVICE_NAME):
	mkdir -p pkg/proto
	protoc -I vendor-proto -I api/proto \
	--go_out pkg/proto --go_opt paths=source_relative \
	--go_opt=Mcommon/common.proto=`go list -m `/pkg/proto/common \
	--go-grpc_out pkg/proto --go-grpc_opt paths=source_relative \
	--grpc-gateway_out pkg/proto --grpc-gateway_opt paths=source_relative \
	--openapiv2_out=json_names_for_fields=false,allow_merge=true,merge_file_name=api:docs \
	api/proto/$(SERVICE_NAME)/*.proto

generate-proto: generate-proto-$(SERVICE_NAME)
