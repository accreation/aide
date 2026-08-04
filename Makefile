.PHONY: build test clean lint build-all

BINARY := aion
ifeq ($(OS),Windows_NT)
	BINARY := aion.exe
endif

build:
	go build -o $(BINARY) .

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
	GOOS=windows GOARCH=amd64 go build -o dist/aion-windows-amd64.exe .
	GOOS=darwin GOARCH=amd64 go build -o dist/aion-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o dist/aion-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -o dist/aion-linux-amd64 .
