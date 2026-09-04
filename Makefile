.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zapier-pp-cli$(BIN_EXT) ./cmd/zapier-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zapier-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zapier-pp-mcp$(BIN_EXT) ./cmd/zapier-pp-mcp

install-mcp:
	go install ./cmd/zapier-pp-mcp

build-all: build build-mcp
