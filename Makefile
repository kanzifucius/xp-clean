MODULE := github.com/kanzifucius/xp-clean
BIN    := bin/xp-clean
VERSION ?= $(shell (git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev) | sed 's/^v//')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build test tidy

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/xp-clean

test:
	go test ./...

tidy:
	go mod tidy
