# xp-clean Helm chart

Minimal chart that installs xp-clean into the dedicated **`xp-clean`** namespace.

**Default path:** a CronJob running `xp-clean run --once` against labelled policy ConfigMaps.

**Optional:** a `manager` Deployment (interval stub) behind `manager.enabled` — off by default.

This chart does **not** ship a CleanupPolicy CRD or claim RBAC.

## Prerequisites

- Helm 3.8+ (OCI support)
- Cluster access to create Namespace / RBAC / CronJob
- Network reachability to `ghcr.io` (the published `ghcr.io/kanzifucius/xp-clean` **image** and `charts/xp-clean` **chart** packages are **public**; anonymous pull)

## Install from GHCR (OCI)

Chart OCI tags are SemVer **without** a leading `v`. Image tags **keep** the leading `v` (GoReleaser). Release packaging sets both from the same git tag.

```bash
helm upgrade --install xp-clean oci://ghcr.io/kanzifucius/charts/xp-clean \
  --version 0.1.0-rc.1 \
  --namespace xp-clean \
  --create-namespace
```

Inspect without installing:

```bash
helm show chart oci://ghcr.io/kanzifucius/charts/xp-clean --version 0.1.0-rc.1
helm template xp-clean oci://ghcr.io/kanzifucius/charts/xp-clean \
  --version 0.1.0-rc.1 --namespace xp-clean
```

## Install from a repository checkout

```bash
helm upgrade --install xp-clean ./deploy/helm/xp-clean \
  --namespace xp-clean \
  --create-namespace \
  --set image.tag=v0.1.0-rc.1
```

From a checkout of this repository the chart path is `deploy/helm/xp-clean`. Prefer pinning `image.tag` to the same SemVer you use for the CLI archive / GHCR image.

For a **private fork** image, create a docker-registry secret and pass `imagePullSecrets` (and set `image.repository` as needed).

## Versioning

| Field | Shape | Example for git tag `v0.1.0-rc.1` |
|-------|--------|-----------------------------------|
| Chart / OCI `--version` | SemVer, no `v` | `0.1.0-rc.1` |
| `appVersion` / `image.tag` | git tag with `v` | `v0.1.0-rc.1` |

On release tags, CI runs `scripts/helm-package.sh` (rewrites `Chart.yaml` + `values.image.tag`, then `helm push` to `oci://ghcr.io/kanzifucius/charts`). Local smoke: `HELM_VERSION=v0.1.0-rc.1 make helm-package` (no push).

## Dry-run default

`dryRun: true` is the chart default. The CronJob runs `xp-clean run --once` as a dry-run plan. Setting chart `dryRun: false` passes `--apply`. Live deletes still require the sample policy ConfigMap `dryRun: false` as well.

## Key values

| Key | Default | Notes |
|-----|---------|-------|
| `namespace.name` | `xp-clean` | Dedicated install namespace |
| `namespace.create` | `true` | Namespace manifest |
| `image.repository` | `ghcr.io/kanzifucius/xp-clean` | Public GHCR image package |
| `image.tag` | `v0.1.0-rc.1` | Prefer immutable `:vX.Y.Z` |
| `imagePullSecrets` | `[]` | Optional; private forks only |
| `dryRun` | `true` | When false, CronJob/manager get `--apply` |
| `cronjob.enabled` | `true` | Default install path |
| `cronjob.schedule` | `0 */6 * * *` | Kubernetes CronJob schedule |
| `manager.enabled` | `false` | Optional secondary path |
| `resources` | small requests/limits | Tune per cluster |
| `policy.create` | `true` | Sample labelled ConfigMap |

See [`values.yaml`](values.yaml) for the full set.

## Render without installing

```bash
helm template xp-clean ./deploy/helm/xp-clean --namespace xp-clean
```

## Uninstall

```bash
helm uninstall xp-clean --namespace xp-clean
# Optional: kubectl delete namespace xp-clean
```
