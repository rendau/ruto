.DEFAULT_GOAL := build

BINARY_NAME = ruto
BUILD_PATH = cmd/build
SERVICE_NAME = ruto_v1

.SILENT:

run:
	go run cmd/main.go

build:
	mkdir -p $(BUILD_PATH)
	CGO_ENABLED=0 go build -o $(BUILD_PATH)/$(BINARY_NAME) cmd/main.go

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
