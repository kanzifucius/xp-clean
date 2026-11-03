MODULE := github.com/kanzifucius/xp-clean
BIN    := bin/xp-clean
VERSION ?= $(shell (git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev) | sed 's/^v//')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

CHART_DIR := deploy/helm/xp-clean
HELM_DEST := dist/helm
# Optional: HELM_VERSION=v0.1.0-rc.2 make helm-package (keep leading v).
# When unset, scripts/helm-package.sh requires an exact SemVer git tag on HEAD.
HELM_VERSION ?=

.PHONY: build test e2e tidy helm-lint helm-template helm-package

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

# Helm chart checks (no registry push). Requires helm on PATH.
helm-lint:
	helm lint $(CHART_DIR)

helm-template:
	helm template xp-clean $(CHART_DIR) --namespace xp-clean >/dev/null

# Package with Chart.yaml / values rewritten from HELM_VERSION or exact git tag. No push.
helm-package:
	@chmod +x scripts/helm-package.sh
	DEST="$(HELM_DEST)" VERSION="$(HELM_VERSION)" ./scripts/helm-package.sh
