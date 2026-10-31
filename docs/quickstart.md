# Quickstart

Build a local binary, point at a sample policy with an **explicit** `--config`, and run a dry-run plan against a cluster that can list XRDs.

## Prerequisites

- Go matching `go.mod` (see repository root)
- A kubeconfig that can list XRDs and the XR GVKs your selectors match

## Build

```bash
make test
make build   # writes bin/xp-clean with version ldflags
```

## Dry-run locally (file override)

There is **no** implicit sample path. Pass `--config` every time you use a file:

```bash
go run ./cmd/xp-clean run --once \
  --config config/samples/cleanuppolicy.yaml \
  --dry-run=true \
  --kubeconfig "$HOME/.kube/config"
```

Inspect the planner output without executing:

```bash
go run ./cmd/xp-clean plan --config config/samples/cleanuppolicy.yaml -o json
```

Trace one XR:

```bash
go run ./cmd/xp-clean trace \
  --name test-db -n xp-test-1 \
  --group db.dev.example.org \
  --kind XDB
```

!!! tip "Live deletes"
    Live deletes require `dryRun: false` in the policy **and** `--dry-run=false`. Prefer dry-run until the plan looks correct. Xp-clean never strips Crossplane finalizers.

## In-cluster sketch

1. Create the dedicated **`xp-clean`** namespace.
2. Deploy a ServiceAccount + RBAC — see [`config/rbac/role.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/rbac/role.yaml) in the repository.
3. Apply one or more labelled policy ConfigMaps (label `xp-clean.io/policy=true`, data key `policy.yaml`). Samples: [`configmap.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap.yaml), [`configmap-team-b.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap-team-b.yaml).
4. Run either:
    - **Job / CronJob:** `xp-clean run --once` (loads ConfigMaps; optional `--config` override)
    - **Deployment:** `xp-clean manager` (interval stub; controller-runtime later)

See [Policy and eligibility](concepts/policy.md) and [Install](install.md) for ConfigMap samples, GHCR pull, and release notes.
