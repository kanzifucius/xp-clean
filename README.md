# xp-clean

[![CI](https://github.com/kanzifucius/xp-clean/actions/workflows/ci.yml/badge.svg)](https://github.com/kanzifucius/xp-clean/actions/workflows/ci.yml)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-teal)](https://kanzifucius.github.io/xp-clean/)
[![GitHub release](https://img.shields.io/github/v/release/kanzifucius/xp-clean?include_prereleases&sort=semver)](https://github.com/kanzifucius/xp-clean/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/kanzifucius/xp-clean)](https://github.com/kanzifucius/xp-clean/blob/main/go.mod)
[![License](https://img.shields.io/github/license/kanzifucius/xp-clean)](LICENSE)

Scheduled discovery and cleanup of Crossplane composition trees (test GVKs and leftover instances).

Xp-clean targets **Crossplane v2-native** stacks only (**Namespaced** / modern **Cluster** XRs). **LegacyCluster claims are not supported** — there is no claim discovery or deletion path.

**Documentation:** [https://kanzifucius.github.io/xp-clean/](https://kanzifucius.github.io/xp-clean/) (Material for MkDocs + mike; enable Pages from the `gh-pages` branch in repository settings after the docs workflow has published).

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

There is **no** snapshot publish from `main`. CI on PRs and `main` runs tests + golangci-lint only.

### CLI on your machine (public one-liner)

Install the latest **non-prerelease** archive into `~/.local/bin` (created if missing; no silent `sudo`):

```bash
curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash
```

Pin a tag, or review the script before running:

```bash
XP_CLEAN_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash
# or:
curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash -s -- --version v0.1.0

curl -fsSL -o install.sh https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh
less install.sh   # review
bash install.sh
```

The installer verifies SHA-256 against the release `checksums.txt`. Optional `GITHUB_TOKEN` / `GH_TOKEN` helps with API rate limits or private forks — not required for the public happy path. Prefer a pinned archive (or `gh release download`) in CI for reproducibility. See [Install and releases](https://kanzifucius.github.io/xp-clean/latest/install/) for PATH notes and GHCR.

### Run in-cluster (private GHCR)

Image tags:

| Tag | Meaning |
|-----|---------|
| `:vX.Y.Z` | Immutable pin (prefer this in clusters) |
| `:X.Y` / `:X` | Floating minor / major on non-prerelease cuts |
| `:latest` | Moved only on **non-prerelease** releases (RCs such as `v0.2.0-rc.1` do not move it) |

The GHCR package stays **private** by preference (even though this GitHub repository is public). Authenticate, then pull:

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

**Precedence:** flags > env (`XP_CLEAN_CONFIG`, `XP_CLEAN_POLICY_NAMESPACE`, `XP_CLEAN_DRY_RUN`, `XP_CLEAN_SCOPE`, `XP_CLEAN_NOTIFY`, `XP_CLEAN_NOTIFY_ON_DRY_RUN`) > policy document > built-in defaults. No Viper. Webhook URLs come only from env vars named by each sink (`XP_CLEAN_TEAMS_WEBHOOK_URL` by default).

| Field / flag | Meaning |
|--------------|---------|
| ConfigMaps / `--config` | In-cluster policies, or explicit file override |
| `--policy-namespace` | Namespace for labelled ConfigMaps (default `xp-clean`) |
| `selectors.requireLabels` / globs | Labels and/or globs (required non-empty) |
| `deletion.scope` / `--scope` | `instances-only` (default) \| `all` |
| `dryRun` / `--dry-run` | Boolean planner dry-run; unset flag keeps policy value (default true) |
| `--fail-on-blocked` | Exit **3** when the run completed with blocked plans |
| `observability.notifications` | Optional Teams (MVP) / Slack (schema-ready) run hooks — see below |
| `--notify`, `--notify-on-dry-run` | Override `notifications.enabled` / `onDryRun` |
| `--kubeconfig`, `--context`, `-n` | kubectl-style access; `-n` is for `trace` only (**rejected** on `run`/`plan`) |

Audit events go to **stdout** JSON only for now — no audit ConfigMap / object-storage retention sink.

### Run notification hooks (Teams Workflows)

Chat summaries sit beside audit JSON and metrics. They wrap each `RunOnce` (started / finished / failed), use a short HTTP deadline, and **never** change cleanup results or exit codes.

1. In Teams, create a **Workflow** (platform-team or channel-owner owned — both are valid; prefer co-owners) with trigger **When a Teams webhook request is received**, then **Post card in a chat or channel**.
2. Copy the Workflow callback URL into a Kubernetes Secret (or local env) as `XP_CLEAN_TEAMS_WEBHOOK_URL`. Never commit the URL to git or put it in the ConfigMap policy body.
3. Enable notifications in the policy ConfigMap / `--config` file:

```yaml
observability:
  notifications:
    enabled: true
    notifyOn: [started, finished, failed]
    onDryRun: true
    sinks:
      - type: teams
        webhookURLFromEnv: XP_CLEAN_TEAMS_WEBHOOK_URL
        logURLTemplate: "https://logs.example/search?q={{runId}}"  # optional
```

4. Mount the Secret as an env var on the Job/CronJob/Deployment and run as usual (`xp-clean run --once` or `manager`). Overrides: `--notify=true` / `XP_CLEAN_NOTIFY=true`, `--notify-on-dry-run` / `XP_CLEAN_NOTIFY_ON_DRY_RUN`.

SSRF host allow-lists are guidance-only for MVP (https-only client; no redirect following). Slack Incoming Webhooks share the same `sinks` list but are not implemented yet.

## CLI

```text
xp-clean run --once [--config PATH] [--policy-namespace NS] [--dry-run=true|false] [--scope instances-only|all] [--fail-on-blocked] [--notify] [--notify-on-dry-run] [--kubeconfig PATH] [--context NAME]
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
- Live deletes honour `deletion.waitTimeout`: after each delete Xp-clean polls until the object is gone (or times out). Stuck finalizers are audited; Crossplane finalizers are **never** stripped.
- With `scope: all`, traces attach CompositionRevision → Composition → XRD; delete order matches that stack. Definition deletes require **zero remaining XRs** for the XRD, and XRD also needs `allowXRDDeletion: true`.
- `xp-clean.io/keep=true` skips the whole trace; paused resources are skipped unless `deletePaused: true`.
- Namespace alone never implies eligibility — use policy labels and/or globs.
- Eligibility never comes from `-n` on `run`/`plan`.

## Licence

Licensed under the [Apache License 2.0](LICENSE).

## Community

- [Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Security](SECURITY.md)
