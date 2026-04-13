.DEFAULT_GOAL := build

BINARY_NAME = ruto
BUILD_PATH = cmd/build

.SILENT:

run:
	go run cmd/main.go

build:
	mkdir -p $(BUILD_PATH)
	CGO_ENABLED=0 go build -o $(BUILD_PATH)/$(BINARY_NAME) cmd/main.go

clean:
	rm -rf $(BUILD_PATH)
