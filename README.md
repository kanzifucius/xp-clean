# xp-clean

Scheduled discovery and cleanup of Crossplane composition trees (test GVKs and leftover instances).

Xp-clean targets **Crossplane v2-native** stacks only (**Namespaced** / modern **Cluster** XRs). **LegacyCluster claims are not supported** — there is no claim discovery or deletion path.

## Features (MVP)

- Discover CompositeResourceDefinitions and build parent→child **traces** (XR → `spec.crossplane.resourceRefs` → children; nested XRs supported).
- Match eligibility with **labels and/or glob patterns** (AND when both are set) via `pkg/match`. GVK globs are **`group/version`** (kind not required).
- Plan deletions with configurable **`deletion.scope`**: `instances-only` (default) or `all`.
- **Dry-run by default** — boolean planner dry-run (client-side `DeletePlan` only; not kubectl API-server dry-run).
- Skip paused resources by default (`crossplane.io/paused`) unless `deletePaused: true`.
- Cobra CLI: `run`, `plan`, `trace`, `manager`, `version`, `completion`.
- Logging via **zap/logr** end-to-end (CLI and manager).

## Build

```bash
make test
make build   # writes bin/xp-clean with version ldflags (Version, GitCommit, BuildDate)
```

## Install / release artefacts

Tagged SemVer releases (`v*`) publish:

1. **GitHub Release archives** — `linux` and `darwin` (`amd64` / `arm64`) plus `checksums.txt` (and archive SBOMs when Syft is available on the runner).
2. **Private GHCR image** — `ghcr.io/kanzifucius/xp-clean` (linux `amd64` / `arm64`).

Image tags:

| Tag | Meaning |
|-----|---------|
| `:vX.Y.Z` | Immutable pin (prefer this in clusters) |
| `:X.Y` / `:X` | Floating minor / major on non-prerelease cuts |
| `:latest` | Moved only on **non-prerelease** releases (RCs such as `v0.2.0-rc.1` do not move it) |

There is **no** snapshot publish from `main`. CI on PRs and `main` runs tests + golangci-lint only.

### Pull a private image

The package stays **private** (matches the private repo). Authenticate, then pull:

```bash
# PAT or fine-grained token with read:packages (SSO-authorise if the org requires it)
echo "$CR_PAT" | docker login ghcr.io -u USERNAME --password-stdin
docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0
```

In-cluster pulls need an `imagePullSecret` whose token can `read:packages` for `ghcr.io/kanzifucius/xp-clean`.

### Cut a release (maintainers)

1. Ensure `main` is green.
2. Tag and push (do not create the Release in the UI):

```bash
git checkout main && git pull
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

3. Watch the `release` workflow; confirm the GitHub Release assets and `docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0`.
4. Spot-check `xp-clean version -o json` from the archive or image (`0.1.0`, matching the tag without the leading `v`).

Local smoke (no push): `goreleaser release --snapshot --clean`.

## Layout

Follows typical Go / Kubernetes operator layout (see architecture plan §11):

| Path | Role |
|------|------|
| `cmd/xp-clean/` | Thin Cobra entrypoints (`main`, `root`, subcommands, `options/`) |
| `internal/cli/` | Cobra-agnostic helpers: kube `clientcmd`, logging, exit codes, policy overrides |
| `internal/conf`, `internal/controller`, `internal/version` | Config load (ConfigMap + file), schedule stub, build metadata |
| `pkg/` | Importable domain library (`discover`, `trace`, `match`, `cleanup`, …) — no Cobra |
| `config/` | Samples + RBAC manifests |

Domain stays in `pkg/` so CLI and a future controller share one `cleanup.Runner`.

## Configuration

**In-cluster primary:** labelled policy **ConfigMaps** in the dedicated **`xp-clean`** namespace (`xp-clean.io/policy=true`, data key `policy.yaml`). Multiple ConfigMaps are supported for per-team multi-tenancy — see [`config/samples/configmap.yaml`](config/samples/configmap.yaml) and [`config/samples/configmap-team-b.yaml`](config/samples/configmap-team-b.yaml).

**File override:** pass explicit **`--config PATH`** (or `XP_CLEAN_CONFIG`). There is **no** implicit repo-relative sample path. A future `CleanupPolicy` CRD may add richer status UX without demoting ConfigMaps.

Sample file document (for `--config` only): [`config/samples/cleanuppolicy.yaml`](config/samples/cleanuppolicy.yaml).

**Precedence:** flags > env (`XP_CLEAN_CONFIG`, `XP_CLEAN_POLICY_NAMESPACE`, `XP_CLEAN_DRY_RUN`, `XP_CLEAN_SCOPE`) > policy document > built-in defaults. No Viper.

| Field / flag | Meaning |
|--------------|---------|
| ConfigMaps / `--config` | In-cluster policies, or explicit file override |
| `--policy-namespace` | Namespace for labelled ConfigMaps (default `xp-clean`) |
| `selectors.requireLabels` / globs | Labels and/or globs (required non-empty) |
| `deletion.scope` / `--scope` | `instances-only` (default) \| `all` |
| `dryRun` / `--dry-run` | Boolean planner dry-run; unset flag keeps policy value (default true) |
| `--fail-on-blocked` | Exit **3** when the run completed with blocked plans |
| `--kubeconfig`, `--context`, `-n` | kubectl-style access; `-n` is for `trace` only (**rejected** on `run`/`plan`) |

Audit events go to **stdout** JSON only for now — no audit ConfigMap / object-storage retention sink.

## CLI

```text
xp-clean run --once [--config PATH] [--policy-namespace NS] [--dry-run=true|false] [--scope instances-only|all] [--fail-on-blocked] [--kubeconfig PATH] [--context NAME]
xp-clean plan [--config PATH] [--policy-namespace NS] [--scope …] [--fail-on-blocked] [-o json|yaml]
xp-clean trace --name ns/name|--name name -n ns --group G --kind K [--version V] [-o text|json|yaml]
xp-clean manager [--config PATH] [--policy-namespace NS]
xp-clean version [-o short|json|yaml]
xp-clean completion bash|zsh|fish|powershell
```

Logging uses zap/logr on stderr (`--zap-devel`, `--zap-log-level`, `--zap-encoder`). Machine output (`plan`, `trace -o json|yaml`, audit JSON on `run`) stays on stdout.

Exit codes: `0` success; `1` runtime failure; `2` usage/validation; `3` blocked plans with `--fail-on-blocked`.

## Run a dry-run locally

Requires a kubeconfig that can list XRDs and the XR GVKs your selectors match. For local file use, pass **`--config` explicitly**:

```bash
go run ./cmd/xp-clean run --once \
  --config config/samples/cleanuppolicy.yaml \
  --dry-run=true \
  --kubeconfig "$HOME/.kube/config"
```

```bash
go run ./cmd/xp-clean plan --config config/samples/cleanuppolicy.yaml -o json
```

```bash
go run ./cmd/xp-clean trace \
  --name test-db -n xp-test-1 \
  --group db.dev.example.org \
  --kind XDB
```

Live deletes require `dryRun: false` in the policy **and** `--dry-run=false`. Prefer dry-run until the plan looks correct. Xp-clean never strips Crossplane finalizers.

## In-cluster sketch

1. Build/push an image containing `xp-clean`.
2. Create the dedicated **`xp-clean`** namespace; deploy a ServiceAccount + RBAC — see [`config/rbac/role.yaml`](config/rbac/role.yaml).
3. Apply one or more labelled policy ConfigMaps into `xp-clean` (see samples).
4. Run either:
   - **Job / CronJob:** `xp-clean run --once` (loads ConfigMaps; optional `--config` override)
   - **Deployment:** `xp-clean manager` (interval stub; controller-runtime later)

## Safety notes

- Default scope deletes **instances only**.
- With `scope: all`, order is CompositionRevision → Composition → XRD; XRD also needs `allowXRDDeletion: true`.
- `xp-clean.io/keep=true` skips the whole trace; paused resources are skipped unless `deletePaused: true`.
- Namespace alone never implies eligibility — use policy labels and/or globs.
- Eligibility never comes from `-n` on `run`/`plan`.

## Licence

Apache-2.0 (or as stated by the repository owner).
