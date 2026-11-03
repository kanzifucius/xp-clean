#!/usr/bin/env bash
# Package the xp-clean Helm chart, rewriting version fields from a SemVer tag.
# Optionally push the packaged chart to GHCR as an OCI artefact.
#
# Usage:
#   ./scripts/helm-package.sh                 # package from exact git tag or VERSION=
#   VERSION=v0.1.0-rc.2 ./scripts/helm-package.sh
#   VERSION=v0.1.0-rc.2 PUSH=1 ./scripts/helm-package.sh
#
# Env:
#   VERSION          Git tag or SemVer (with or without leading v). Required unless
#                    HEAD is an exact annotated/lightweight SemVer tag.
#   PUSH             Set to 1 to helm push after packaging (default: 0).
#   DEST             Output directory for the .tgz (default: dist/helm).
#   OCI_REGISTRY     OCI registry prefix without chart name
#                    (default: oci://ghcr.io/kanzifucius/charts).
#   CHART_DIR        Chart source path (default: deploy/helm/xp-clean).
#
# Chart OCI tag = SemVer without leading v.
# appVersion and values.image.tag keep the leading v to match GoReleaser image tags.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${CHART_DIR:-deploy/helm/xp-clean}"
DEST="${DEST:-dist/helm}"
OCI_REGISTRY="${OCI_REGISTRY:-oci://ghcr.io/kanzifucius/charts}"
PUSH="${PUSH:-0}"

log() { printf '==> %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }

need helm

resolve_tag() {
  local raw=""
  if [[ -n "${VERSION:-}" ]]; then
    raw="${VERSION}"
  elif [[ -n "${GITHUB_REF_NAME:-}" && "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
    raw="${GITHUB_REF_NAME}"
  else
    raw="$(git -C "${ROOT}" describe --tags --exact-match 2>/dev/null || true)"
  fi
  [[ -n "${raw}" ]] || die "set VERSION=vX.Y.Z (or run on an exact SemVer git tag)"

  # Accept v0.1.0 / 0.1.0 / v0.1.0-rc.2
  if [[ "${raw}" == v* ]]; then
    GIT_TAG="${raw}"
  else
    GIT_TAG="v${raw}"
  fi
  CHART_VERSION="${GIT_TAG#v}"

  # SemVer with optional -prerelease (no build metadata / no git-describe suffixes).
  [[ "${CHART_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] \
    || die "refusing non-SemVer VERSION '${raw}' (got chart version '${CHART_VERSION}')"
}

set_chart_field() {
  # set_chart_field <file> <key> <value> — rewrite a top-level YAML scalar key.
  # awk-only so we do not depend on mikefarah vs kislyuk yq variants.
  local file="$1" key="$2" value="$3"
  local tmp
  tmp="$(mktemp)"
  awk -v k="${key}" -v v="${value}" '
    BEGIN { done = 0 }
    $0 ~ "^" k ":" && !done {
      print k ": \"" v "\""
      done = 1
      next
    }
    { print }
  ' "${file}" >"${tmp}"
  mv "${tmp}" "${file}"
}

set_image_tag() {
  local file="$1" tag="$2"
  local tmp
  tmp="$(mktemp)"
  awk -v tag="${tag}" '
    BEGIN { in_image = 0; done = 0 }
    /^image:/ { in_image = 1; print; next }
    in_image && /^[^ \t]/ { in_image = 0 }
    in_image && /^[ \t]+tag:/ && !done {
      match($0, /^[ \t]+/)
      print substr($0, 1, RLENGTH) "tag: \"" tag "\""
      done = 1
      next
    }
    { print }
  ' "${file}" >"${tmp}"
  mv "${tmp}" "${file}"
}

resolve_tag
log "packaging chart version=${CHART_VERSION} appVersion/image.tag=${GIT_TAG}"

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
cp -a "${ROOT}/${CHART_DIR}" "${work}/xp-clean"

set_chart_field "${work}/xp-clean/Chart.yaml" version "${CHART_VERSION}"
set_chart_field "${work}/xp-clean/Chart.yaml" appVersion "${GIT_TAG}"
set_image_tag "${work}/xp-clean/values.yaml" "${GIT_TAG}"

mkdir -p "${ROOT}/${DEST}"
# helm package --version/--app-version also override Chart.yaml; keep both for clarity.
helm package "${work}/xp-clean" \
  --version "${CHART_VERSION}" \
  --app-version "${GIT_TAG}" \
  --destination "${ROOT}/${DEST}"

tgz="${ROOT}/${DEST}/xp-clean-${CHART_VERSION}.tgz"
[[ -f "${tgz}" ]] || die "expected package missing: ${tgz}"
log "wrote ${tgz}"

if [[ "${PUSH}" == "1" ]]; then
  need helm
  log "pushing to ${OCI_REGISTRY} (lands at ${OCI_REGISTRY}/xp-clean:${CHART_VERSION})"
  helm push "${tgz}" "${OCI_REGISTRY}"
fi

printf '%s\n' "${tgz}"
