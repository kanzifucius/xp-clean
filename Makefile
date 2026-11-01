MODULE := github.com/kanzifucius/xp-clean
BIN    := bin/xp-clean
VERSION ?= $(shell (git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev) | sed 's/^v//')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build test e2e tidy

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/xp-clean

test:
	go test ./...

# Kind e2e (local-first). Requires Docker, kind, kubectl, helm.
# Not part of the required CI job named `test` — see test/e2e/README.md.
e2e: build
	@chmod +x test/e2e/run-kind.sh
	./test/e2e/run-kind.sh

tidy:
	go mod tidy
