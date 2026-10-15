BINARY := bin/azt
# Minimum Go version from go.mod; test-min-go checks the code still builds on it.
MIN_GO := $(shell go list -m -f '{{.GoVersion}}')

.PHONY: check fmt vet lint test test-min-go build run clean

check: fmt vet lint test test-min-go build

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test ./...

test-min-go:
	GOTOOLCHAIN=go$(MIN_GO) go test ./...

build:
	go build -o $(BINARY) ./cmd/azt

run:
	go run ./cmd/azt

clean:
	rm -rf bin
