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

There is **no** implicit sample path. Pass `--config` every time you use a file. `run --once` defaults to a dry-run plan (stacks matched / would delete):

```bash
go run ./cmd/xp-clean run --once \
  --config config/samples/cleanuppolicy.yaml \
  --kubeconfig "$HOME/.kube/config"
```

Scripts that need machine-readable DeletePlans can still use the thin deprecated wrapper:

```bash
go run ./cmd/xp-clean plan --config config/samples/cleanuppolicy.yaml -o json
```

Inspect one composition tree (debug only):

```bash
go run ./cmd/xp-clean debug trace \
  --name test-db -n xp-test-1 \
  --group db.dev.example.org \
  --kind XDB
```

!!! tip "Live deletes"
    Live deletes require **`--apply`** and policy `dryRun: false`. Deletes use **foreground** propagation. Waiting is opt-in (`--wait`). Prefer the default dry-run roots plan until stacks matched / would delete looks correct. Use `--mode=trace` for full trees. Xp-clean never strips Crossplane finalizers.

## In-cluster (Helm)

Prefer the published OCI chart (chart `--version` omits the leading `v`):

```bash
helm upgrade --install xp-clean oci://ghcr.io/kanzifucius/charts/xp-clean \
  --version 0.1.0-rc.1 \
  --namespace xp-clean \
  --create-namespace
```

From a checkout use [`deploy/helm/xp-clean`](https://github.com/kanzifucius/xp-clean/tree/main/deploy/helm/xp-clean) and pin `--set image.tag=v0.1.0-rc.1`. That installs Namespace, ServiceAccount, RBAC, a sample policy ConfigMap (label eligibility + TTL + dry-run + instances-only), and a CronJob (`xp-clean run --once`) with **dry-run on by default**. The optional manager Deployment stays off unless `manager.enabled=true`.

Without Helm, apply [`config/rbac/role.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/rbac/role.yaml) plus labelled ConfigMaps (`xp-clean.io/policy=true`, data key `policy.yaml`) from [`config/samples/`](https://github.com/kanzifucius/xp-clean/tree/main/config/samples).

See [Policy and eligibility](concepts/policy.md) and [Install](install.md) for values, GHCR pull, and release notes.
