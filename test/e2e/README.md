# End-to-end tests

Xp-clean has two e2e layers:

| Layer | How to run | CI |
|-------|------------|----|
| **Pipeline** (`pipeline_test.go`) | `go test ./test/e2e/...` (also via `make test`) | Yes — part of required job **`test`** |
| **Kind** (`run-kind.sh`) | `make e2e` | **No** (local-first; needs Docker, kind, helm) |

## Pipeline tests

In-memory `MemoryBackend` scenarios for XR→ConfigMap/Deployment trees:

- dry-run plan does not delete
- live instances-only XR delete
- `xp-clean.io/keep` skips planning
- `crossplane.io/paused` blocks the plan
- `--fail-on-blocked` maps to exit code **3**

No claim fixtures. No cloud-provider managed resources.

## Kind e2e

Prerequisites: Docker, [kind](https://kind.sigs.k8s.io/), kubectl, helm, Go matching `go.mod`.

```bash
make e2e
# or:
./test/e2e/run-kind.sh
```

What it does:

1. Creates (or reuses) kind cluster `xp-clean-e2e`
2. Installs Crossplane (default chart `1.20.1`; override with `XP_CLEAN_E2E_CROSSPLANE_VERSION`)
3. Applies XR-only XRD + seeded XR/ConfigMap/Deployment fixtures under `fixtures/`
4. Asserts dry-run plan, keep, paused + `--fail-on-blocked` (exit 3), and live XR delete

Useful env vars:

| Variable | Meaning |
|----------|---------|
| `XP_CLEAN_E2E_KEEP_CLUSTER=1` | Leave the kind cluster after success |
| `XP_CLEAN_E2E_SKIP_CLUSTER=1` | Use existing `KUBECONFIG` / `XP_CLEAN_E2E_KUBECONFIG` |
| `XP_CLEAN_E2E_SKIP_CROSSPLANE=1` | Skip helm install (Crossplane already present) |
| `XP_CLEAN_E2E_CLUSTER` | Kind cluster name (default `xp-clean-e2e`) |

**Note:** Live delete wait/finalizer polling and `scope: all` definition attachment are covered more fully after the XP-26 waitTimeout work lands; this suite still validates instances-only XR delete against a real API server. Rebase kind assertions onto that PR if `LiveBackend` / `AttachDefinitions` conflict.
