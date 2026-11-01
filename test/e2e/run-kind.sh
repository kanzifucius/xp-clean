#!/usr/bin/env bash
# Kind e2e for xp-clean: dry-run plan, keep/paused gates, --fail-on-blocked, live XR delete.
# Local-first — not wired into the required CI job `test` (needs Docker + kind).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER_NAME="${XP_CLEAN_E2E_CLUSTER:-xp-clean-e2e}"
CROSSPLANE_VERSION="${XP_CLEAN_E2E_CROSSPLANE_VERSION:-1.20.1}"
KUBECONFIG_PATH="${XP_CLEAN_E2E_KUBECONFIG:-${ROOT}/bin/e2e.kubeconfig}"
BIN="${ROOT}/bin/xp-clean"
FIXTURES="${ROOT}/test/e2e/fixtures"
POLICY="${FIXTURES}/policy-e2e.yaml"

log() { printf '==> %s\n' "$*"; }
need() { command -v "$1" >/dev/null 2>&1 || { echo "missing required tool: $1" >&2; exit 2; }; }

need kind
need kubectl
need helm
need go

mkdir -p "${ROOT}/bin"

if [[ "${XP_CLEAN_E2E_SKIP_CLUSTER:-}" != "1" ]]; then
  if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
    log "creating kind cluster ${CLUSTER_NAME}"
    kind create cluster --name "${CLUSTER_NAME}" --kubeconfig "${KUBECONFIG_PATH}"
  else
    log "reusing kind cluster ${CLUSTER_NAME}"
    kind export kubeconfig --name "${CLUSTER_NAME}" --kubeconfig "${KUBECONFIG_PATH}"
  fi
fi
export KUBECONFIG="${KUBECONFIG_PATH}"

if [[ "${XP_CLEAN_E2E_SKIP_CROSSPLANE:-}" != "1" ]]; then
  log "installing Crossplane ${CROSSPLANE_VERSION}"
  helm repo add crossplane-stable https://charts.crossplane.io/stable >/dev/null 2>&1 || true
  helm repo update crossplane-stable >/dev/null
  helm upgrade --install crossplane crossplane-stable/crossplane \
    --namespace crossplane-system --create-namespace \
    --version "${CROSSPLANE_VERSION}" \
    --wait --timeout 5m
  kubectl wait --for=condition=Available deployment/crossplane -n crossplane-system --timeout=5m
fi

log "applying XRD"
kubectl apply -f "${FIXTURES}/xdb-xrd.yaml"
# Composition fixture is documentary only (may need functions); not required for instances-only e2e.

log "waiting for XDB CRD"
for _ in $(seq 1 60); do
  if kubectl get crd xdbs.db.dev.example.org >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
kubectl wait --for=condition=Established "crd/xdbs.db.dev.example.org" --timeout=120s

log "building xp-clean"
make -C "${ROOT}" build

log "applying XR fixtures (ConfigMap + Deployment compositions; no cloud providers)"
kubectl apply -f "${FIXTURES}/instances-eligible.yaml"
kubectl apply -f "${FIXTURES}/instances-keep.yaml"
kubectl apply -f "${FIXTURES}/instances-paused.yaml"

log "scenario: dry-run plan"
OUT="$("${BIN}" plan --config "${POLICY}" --kubeconfig "${KUBECONFIG}" -o json)"
echo "${OUT}" | grep -q 'test-db' || { echo "dry-run plan missing test-db"; echo "${OUT}"; exit 1; }
kubectl get xdb test-db -n xp-test-1 >/dev/null

log "scenario: keep gate — keep-db must remain after a live run"
# Narrow live policy so only keep-* is considered; DiscoverRoots drops keep labels.
KEEP_POLICY="$(mktemp)"
cat > "${KEEP_POLICY}" <<'EOF'
name: e2e-keep
dryRun: false
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    namespaces: ["xp-test-keep"]
    xrNames: ["keep-*"]
ttl:
  respectResourceTTL: false
deletion:
  scope: instances-only
  deletePaused: false
  allowXRDDeletion: false
  waitTimeout: 2m
observability:
  verbosity: normal
  notifications:
    enabled: false
EOF
"${BIN}" run --once --config "${KEEP_POLICY}" --dry-run=false --kubeconfig "${KUBECONFIG}"
kubectl get xdb keep-db -n xp-test-keep >/dev/null
rm -f "${KEEP_POLICY}"

log "scenario: paused + --fail-on-blocked → exit 3"
PAUSED_POLICY="$(mktemp)"
cat > "${PAUSED_POLICY}" <<'EOF'
name: e2e-paused
dryRun: false
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    namespaces: ["xp-test-paused"]
    xrNames: ["paused-*"]
ttl:
  respectResourceTTL: false
deletion:
  scope: instances-only
  deletePaused: false
  allowXRDDeletion: false
  waitTimeout: 2m
observability:
  verbosity: normal
  notifications:
    enabled: false
EOF
set +e
"${BIN}" run --once --config "${PAUSED_POLICY}" --dry-run=false --fail-on-blocked --kubeconfig "${KUBECONFIG}"
RC=$?
set -e
rm -f "${PAUSED_POLICY}"
if [[ "${RC}" -ne 3 ]]; then
  echo "expected exit 3 for --fail-on-blocked, got ${RC}" >&2
  exit 1
fi
kubectl get xdb paused-db -n xp-test-paused >/dev/null

log "scenario: live XR delete (instances-only)"
LIVE_POLICY="$(mktemp)"
cat > "${LIVE_POLICY}" <<'EOF'
name: e2e-live
dryRun: false
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    namespaces: ["xp-test-1"]
    xrNames: ["test-*"]
ttl:
  respectResourceTTL: false
deletion:
  scope: instances-only
  deletePaused: false
  allowXRDDeletion: false
  waitTimeout: 2m
observability:
  verbosity: normal
  notifications:
    enabled: false
EOF
kubectl apply -f "${FIXTURES}/instances-eligible.yaml"
"${BIN}" run --once --config "${LIVE_POLICY}" --dry-run=false --kubeconfig "${KUBECONFIG}"
rm -f "${LIVE_POLICY}"

if kubectl get xdb test-db -n xp-test-1 >/dev/null 2>&1; then
  sleep 2
  if kubectl get xdb test-db -n xp-test-1 >/dev/null 2>&1; then
    echo "expected test-db XR to be deleted" >&2
    exit 1
  fi
fi
# Composed children remain under instances-only until Crossplane GC (waitTimeout wiring is XP-26).
kubectl get configmap test-db-cm -n xp-test-1 >/dev/null
kubectl get deploy test-db-deploy -n xp-test-1 >/dev/null

log "kind e2e passed"
if [[ "${XP_CLEAN_E2E_KEEP_CLUSTER:-}" != "1" ]]; then
  log "deleting kind cluster ${CLUSTER_NAME} (set XP_CLEAN_E2E_KEEP_CLUSTER=1 to retain)"
  kind delete cluster --name "${CLUSTER_NAME}" || true
  rm -f "${KUBECONFIG_PATH}"
fi
