.PHONY: build test clean lint build-all

VERSION ?= 0.1.0
LDFLAGS := -X aide/cmd.Version=$(VERSION)

BINARY := aide
ifeq ($(OS),Windows_NT)
	BINARY := aide.exe
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
	rm -f aide aide.exe

fmt:
	go fmt ./internal/... ./cmd/...

dist:
	mkdir -p dist

build-all: dist
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/aide-windows-amd64.exe .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/aide-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/aide-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/aide-linux-amd64 .
