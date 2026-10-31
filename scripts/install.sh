#!/bin/sh
# xp-clean CLI installer — download a GitHub Release archive, verify SHA-256,
# and install the binary into a user-writable directory (default: ~/.local/bin).
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash
#
# Pin a tag:
#   XP_CLEAN_VERSION=v0.1.0 curl -fsSL …/scripts/install.sh | bash
#   curl -fsSL …/scripts/install.sh | bash -s -- --version v0.1.0
#
# Optional: GITHUB_TOKEN / GH_TOKEN for rate limits, GHES, or private forks.
# Never echo the token. Prefer reviewing this script before piping to a shell.

set -eu

OWNER="kanzifucius"
REPO="xp-clean"
BINARY="xp-clean"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

log() { printf '%s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }

usage() {
  cat >&2 <<'EOF'
Install xp-clean from GitHub Releases.

Usage:
  install.sh [--version TAG] [--install-dir DIR]

Environment:
  XP_CLEAN_VERSION      Pin release tag (e.g. v0.1.0); default is latest non-prerelease
  XP_CLEAN_INSTALL_DIR  Install directory (default: ~/.local/bin)
  BINDIR                Alias for XP_CLEAN_INSTALL_DIR
  GITHUB_TOKEN / GH_TOKEN  Optional auth for GitHub API / downloads

Flags:
  --version TAG         Same as XP_CLEAN_VERSION
  --install-dir DIR     Same as XP_CLEAN_INSTALL_DIR
  -h, --help            Show this help
EOF
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# Auth header file (if any) so the token is not required on curl argv forever.
AUTH_HEADER_FILE=""
cleanup_auth() {
  if [ -n "${AUTH_HEADER_FILE}" ] && [ -f "${AUTH_HEADER_FILE}" ]; then
    rm -f "${AUTH_HEADER_FILE}"
  fi
}

prepare_auth() {
  token="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
  if [ -z "${token}" ]; then
    return 0
  fi
  # Restrictive mode before mktemp; contents hold a secret.
  old_umask="$(umask)"
  umask 077
  AUTH_HEADER_FILE="$(mktemp "${TMPDIR:-/tmp}/xp-clean-auth.XXXXXX")"
  umask "${old_umask}"
  printf 'Authorization: Bearer %s\n' "${token}" >"${AUTH_HEADER_FILE}"
  unset token
}

http_get() {
  # http_get URL OUTFILE
  url="$1"
  out="$2"
  if [ -n "${AUTH_HEADER_FILE}" ]; then
    curl -fsSL -H @"${AUTH_HEADER_FILE}" -o "${out}" "${url}"
  else
    curl -fsSL -o "${out}" "${url}"
  fi
}

http_get_body() {
  # http_get_body URL → stdout
  url="$1"
  if [ -n "${AUTH_HEADER_FILE}" ]; then
    curl -fsSL -H @"${AUTH_HEADER_FILE}" "${url}"
  else
    curl -fsSL "${url}"
  fi
}

detect_platform() {
  os="$(uname -s)"
  arch="$(uname -m)"

  case "${os}" in
    Linux) OS_NAME="Linux" ;;
    Darwin) OS_NAME="Darwin" ;;
    *) die "unsupported OS: ${os} (need Linux or Darwin)" ;;
  esac

  case "${arch}" in
    x86_64 | amd64) ARCH_NAME="x86_64" ;;
    aarch64 | arm64) ARCH_NAME="arm64" ;;
    *) die "unsupported architecture: ${arch} (need x86_64/amd64 or arm64/aarch64)" ;;
  esac
}

normalise_tag() {
  # Accept 0.1.0 or v0.1.0; emit v0.1.0 for GitHub tag URLs.
  tag="$1"
  case "${tag}" in
    v*) printf '%s\n' "${tag}" ;;
    *) printf 'v%s\n' "${tag}" ;;
  esac
}

resolve_version() {
  if [ -n "${VERSION_ARG}" ]; then
    TAG="$(normalise_tag "${VERSION_ARG}")"
    VERSION="${TAG#v}"
    return 0
  fi

  log "resolving latest non-prerelease from GitHub…"
  # releases/latest excludes drafts and prereleases.
  api="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
  body="$(http_get_body "${api}")" || die "failed to query ${api}"
  TAG="$(printf '%s\n' "${body}" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  [ -n "${TAG}" ] || die "could not parse tag_name from releases/latest (no release yet?)"
  VERSION="${TAG#v}"
}

sha256_file() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file}" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "${file}" | awk '{print $NF}'
  else
    die "no SHA-256 tool found (need sha256sum, shasum, or openssl)"
  fi
}

verify_checksum() {
  archive="$1"
  checksums="$2"
  base="$(basename "${archive}")"
  expected="$(awk -v f="${base}" '$2 == f { print $1; exit }' "${checksums}")"
  [ -n "${expected}" ] || die "no checksum entry for ${base} in checksums.txt"
  actual="$(sha256_file "${archive}")"
  [ "${actual}" = "${expected}" ] || die "checksum mismatch for ${base}: got ${actual}, want ${expected}"
  log "checksum OK for ${base}"
}

path_has_dir() {
  dir="$1"
  case ":${PATH}:" in
    *":${dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

# All destructive work lives here so a truncated curl|bash stream fails closed
# (incomplete function / syntax error) rather than half-installing.
execute() {
  tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/xp-clean-install.XXXXXX")"
  trap 'rm -rf "${tmpdir}"; cleanup_auth' EXIT INT HUP TERM

  archive_name="${BINARY}_${VERSION}_${OS_NAME}_${ARCH_NAME}.tar.gz"
  base_url="https://github.com/${OWNER}/${REPO}/releases/download/${TAG}"
  archive_url="${base_url}/${archive_name}"
  checksums_url="${base_url}/checksums.txt"

  log "downloading ${archive_name} (${TAG})…"
  http_get "${checksums_url}" "${tmpdir}/checksums.txt" \
    || die "failed to download checksums.txt from ${checksums_url}"
  http_get "${archive_url}" "${tmpdir}/${archive_name}" \
    || die "failed to download ${archive_name} from ${archive_url}"

  verify_checksum "${tmpdir}/${archive_name}" "${tmpdir}/checksums.txt"

  tar -xzf "${tmpdir}/${archive_name}" -C "${tmpdir}"
  [ -f "${tmpdir}/${BINARY}" ] || die "archive did not contain ${BINARY}"

  if [ ! -d "${INSTALL_DIR}" ]; then
    mkdir -p "${INSTALL_DIR}" || die "could not create install directory: ${INSTALL_DIR}"
  fi
  if [ ! -w "${INSTALL_DIR}" ]; then
    die "install directory is not writable: ${INSTALL_DIR}
Refusing to run sudo. Re-run with a writable --install-dir, or install manually:
  sudo install -m 0755 ${tmpdir}/${BINARY} ${INSTALL_DIR}/${BINARY}"
  fi

  install -m 0755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  dest="${INSTALL_DIR}/${BINARY}"
  log "installed ${dest}"

  if ! path_has_dir "${INSTALL_DIR}"; then
    log "Add ${INSTALL_DIR} to your PATH, for example:"
    log "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  fi

  if [ -x "${dest}" ]; then
    # Prefer the installed path so PATH order cannot hide a stale binary.
    ver_out="$("${dest}" version -o short 2>/dev/null || "${dest}" version 2>/dev/null || true)"
    if [ -n "${ver_out}" ]; then
      log "version: ${ver_out}"
    else
      log "installed successfully (could not run '${BINARY} version')"
    fi
  fi
}

# --- argument parsing (POSIX) ---
VERSION_ARG="${XP_CLEAN_VERSION:-}"
INSTALL_DIR="${XP_CLEAN_INSTALL_DIR:-${BINDIR:-${DEFAULT_INSTALL_DIR}}}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || die "--version requires a tag"
      VERSION_ARG="$2"
      shift 2
      ;;
    --version=*)
      VERSION_ARG="${1#--version=}"
      shift
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || die "--install-dir requires a path"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --install-dir=*)
      INSTALL_DIR="${1#--install-dir=}"
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1 (try --help)"
      ;;
  esac
done

need_cmd curl
need_cmd tar
need_cmd install
need_cmd mktemp
need_cmd uname
need_cmd sed
need_cmd awk
need_cmd head

# Fail early if no hash tool exists (do not skip verification).
if ! command -v sha256sum >/dev/null 2>&1 \
  && ! command -v shasum >/dev/null 2>&1 \
  && ! command -v openssl >/dev/null 2>&1; then
  die "no SHA-256 tool found (need sha256sum, shasum, or openssl)"
fi

prepare_auth
trap 'cleanup_auth' EXIT INT HUP TERM

detect_platform
resolve_version
log "installing ${BINARY} ${TAG} for ${OS_NAME}/${ARCH_NAME} → ${INSTALL_DIR}"
execute
