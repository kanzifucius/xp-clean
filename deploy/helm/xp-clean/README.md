# xp-clean Helm chart

Minimal chart that installs xp-clean into the dedicated **`xp-clean`** namespace.

**Default path:** a CronJob running `xp-clean run --once` against labelled policy ConfigMaps.

**Optional:** a `manager` Deployment (interval stub) behind `manager.enabled` — off by default.

This chart does **not** ship a CleanupPolicy CRD or claim RBAC.

## Prerequisites

- Helm 3
- Cluster access to create Namespace / RBAC / CronJob
- Network reachability to `ghcr.io` (the published `ghcr.io/kanzifucius/xp-clean` package is **public**; anonymous pull)

## Install

```bash
helm upgrade --install xp-clean ./deploy/helm/xp-clean \
  --namespace xp-clean \
  --create-namespace \
  --set image.tag=v0.1.0-rc.1
```

From a checkout of this repository the chart path is `deploy/helm/xp-clean`. Pin `image.tag` to the same SemVer you use for the CLI archive.

For a **private fork** image, create a docker-registry secret and pass `imagePullSecrets` (and set `image.repository` as needed).

## Dry-run default

`dryRun: true` is the chart default (CLI `--dry-run=true`). The sample policy ConfigMap also has `dryRun: true`. Live deletes require **both** the chart value and the policy document set to `false`.

## Key values

| Key | Default | Notes |
|-----|---------|-------|
| `namespace.name` | `xp-clean` | Dedicated install namespace |
| `namespace.create` | `true` | Namespace manifest |
| `image.repository` | `ghcr.io/kanzifucius/xp-clean` | Public GHCR package |
| `image.tag` | `v0.1.0-rc.1` | Prefer immutable `:vX.Y.Z` |
| `imagePullSecrets` | `[]` | Optional; private forks only |
| `dryRun` | `true` | Passed as `--dry-run` |
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
