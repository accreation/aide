.PHONY: build test clean lint build-all

VERSION ?= 0.1.0
LDFLAGS := -X aion/cmd.Version=$(VERSION)

BINARY := aion
ifeq ($(OS),Windows_NT)
	BINARY := aion.exe
endif

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./internal/...

test-race:
	go test -race ./internal/...

lint:
	go vet ./internal/...

clean:
	rm -f aion aion.exe

fmt:
	go fmt ./internal/... ./cmd/...

dist:
	mkdir -p dist

build-all: dist
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/aion-windows-amd64.exe .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/aion-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/aion-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/aion-linux-amd64 .
