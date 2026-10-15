BINARY  := azt
PKG     := github.com/ingvarch/azt
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
# The oldest Go azt supports, from go.mod; test-min-go runs the tests on it.
MIN_GO  := $(shell go list -m -f '{{.GoVersion}}')

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.Date=$(DATE)

.PHONY: check fmt vet lint licenses notices test test-min-go build dist run clean

check: fmt vet lint licenses test test-min-go build

fmt:
	gofmt -l -w .

vet:
	go vet ./...

# The version CI lints with: another one finds other things.
GOLANGCI_LINT_VERSION := 2.14.0

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed: CI lints with v$(GOLANGCI_LINT_VERSION)"; \
		exit 1; \
	}
	@installed=$$(golangci-lint version --short); \
	if [ "$$installed" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint $$installed is installed: CI lints with v$(GOLANGCI_LINT_VERSION)"; \
		exit 1; \
	fi
	golangci-lint run ./...

# licenses fails when a module azt links, on any platform the release builds
# for, has a license outside internal/licenses.Allowed. notices checks the
# same and writes THIRD_PARTY_NOTICES, which the release ships.
licenses:
	go run ./internal/licenses/cmd/licenses ./cmd/$(BINARY)

notices:
	go run ./internal/licenses/cmd/licenses -notices THIRD_PARTY_NOTICES ./cmd/$(BINARY)

test:
	go test ./...

test-min-go:
	GOTOOLCHAIN=go$(MIN_GO) go test ./...

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

# dist builds every platform and package the way a release does and publishes
# nothing. It needs goreleaser (.tool-versions) and syft; signing needs the
# release workflow, so it is skipped.
dist:
	goreleaser release --snapshot --clean --skip=sign

run: build
	./bin/$(BINARY)

clean:
	rm -rf bin dist THIRD_PARTY_NOTICES
