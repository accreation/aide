.PHONY: build test clean lint

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
