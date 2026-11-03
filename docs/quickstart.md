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

## In-cluster (Helm)

Prefer the chart at [`deploy/helm/xp-clean`](https://github.com/kanzifucius/xp-clean/tree/main/deploy/helm/xp-clean):

```bash
helm upgrade --install xp-clean ./deploy/helm/xp-clean \
  --namespace xp-clean \
  --set image.tag=v0.1.0-rc.1 \
  --set imagePullSecrets[0].name=ghcr-xp-clean
```

That installs Namespace, ServiceAccount, RBAC, a sample policy ConfigMap, and a CronJob (`xp-clean run --once`) with **dry-run on by default**. The optional manager Deployment stays off unless `manager.enabled=true`.

Without Helm, apply [`config/rbac/role.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/rbac/role.yaml) plus labelled ConfigMaps (`xp-clean.io/policy=true`, data key `policy.yaml`) from [`config/samples/`](https://github.com/kanzifucius/xp-clean/tree/main/config/samples).

See [Policy and eligibility](concepts/policy.md) and [Install](install.md) for values, GHCR pull, and release notes.
