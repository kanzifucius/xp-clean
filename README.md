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

- Discover CompositeResourceDefinitions and select **highest-parent** XRs (**roots** mode default). Full parent→child **traces** are opt-in (`--mode=trace`).
- Match eligibility with **labels** (day-1) and/or **glob patterns** (advanced) via `pkg/match`. GVK globs are **`group/version`** (kind not required).
- Plan deletions with **`deletion.scope: instances-only`** by default (`all` is advanced).
- **Dry-run by default** — `xp-clean run --once` prints stacks matched / would delete; live needs `--apply` and policy `dryRun: false`.
- Live Deletes use **foreground** propagation; **wait** after delete is opt-in (`--wait` / `deletion.wait`).
- Skip paused resources by default (`crossplane.io/paused`) unless `deletePaused: true`.
- Cobra CLI: `run` (day-to-day), deprecated `plan`, `debug trace`, `manager`, `skill install`, `version`, `completion`.
- Logging via **zap/logr** end-to-end (CLI and manager).
- **`xp-clean skill install`** embeds a Cursor skill (`~/.cursor/skills/xp-clean/` by default; `--project` for repo-local) that teaches dry-run first, confirm before `--apply`, and `--gvk-glob`.

## Build

```bash
make test
make build   # writes bin/xp-clean with version ldflags (Version, GitCommit, BuildDate)
```

Unit tests cover `pkg/discover` and an in-memory e2e pipeline under `test/e2e`. For a kind cluster (Crossplane + XR→ConfigMap/Deployment fixtures):

```bash
make e2e   # local-first; not part of the required CI job named test — see test/e2e/README.md
```

## Install / release artefacts

Tagged SemVer releases (`v*`) publish:

1. **GitHub Release archives** — `linux` and `darwin` (`amd64` / `arm64`) plus `checksums.txt` (and archive SBOMs when Syft is available on the runner).
2. **GHCR image** — `ghcr.io/kanzifucius/xp-clean` (linux `amd64` / `arm64`). The package is **public** (anonymous pull); that is the locked preference.

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

Install the Cursor skill after the binary is on your `PATH`:

```bash
xp-clean skill install            # ~/.cursor/skills/xp-clean/SKILL.md
xp-clean skill install --project  # repo-local .cursor/skills/xp-clean/
```

### Run in-cluster (GHCR)

Image tags:

| Tag | Meaning |
|-----|---------|
| `:vX.Y.Z` / `:vX.Y.Z-rc.N` | Immutable pin (prefer this in clusters; RCs publish **only** this tag) |
| `:X.Y` / `:X` | Floating minor / major — moved only on **non-prerelease** cuts |
| `:latest` | Moved only on **non-prerelease** releases (RCs such as `v0.2.0-rc.1` do not move it) |

The container package at [`ghcr.io/kanzifucius/xp-clean`](https://github.com/kanzifucius/xp-clean/pkgs/container/xp-clean) is **public**, so cluster nodes can pull without an `imagePullSecret`:

```bash
docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0-rc.1
```

Private forks still need a PAT with `read:packages` and an `imagePullSecret` — see [Install and releases](https://kanzifucius.github.io/xp-clean/latest/install/).

### Cut a release (maintainers)

1. Ensure `main` is green.
2. Tag and push (do not create the Release in the UI):

```bash
git checkout main && git pull
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

3. Watch the `release` workflow; confirm the GitHub Release assets, `docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0`, and `helm show chart oci://ghcr.io/kanzifucius/charts/xp-clean --version 0.1.0`.
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
| `selectors.requireLabels` | Day-1 eligibility (globs are advanced) |
| `deletion.mode` / `--mode` | `roots` (default) \| `trace` (full composition trees) |
| `deletion.scope` / `--scope` | `instances-only` (default) \| `all` (advanced) |
| `deletion.wait` / `--wait` | Opt-in poll-until-gone (default off) |
| `dryRun` / `--apply` | Policy dry-run gate; CLI defaults to dry-run plan; `--apply` for live (still needs policy `dryRun: false`) |
| `--dry-run` | Compatibility boolean; prefer `--apply` for live |
| `--fail-on-blocked` | Exit **3** when the run completed with blocked plans |
| `observability.notifications` | Advanced: optional Teams (MVP) / Slack (schema-ready) — see below |
| `--notify`, `--notify-on-dry-run` | Override `notifications.enabled` / `onDryRun` |
| `--kubeconfig`, `--context`, `-n` | kubectl-style access; `-n` is for `debug trace` only (**rejected** on `run`/`plan`) |

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
xp-clean run --once [--config PATH] [--policy-namespace NS] [--apply] [--mode roots|trace] [--wait] [--dry-run=true|false] [--scope instances-only|all] [--fail-on-blocked] [--notify] [--notify-on-dry-run] [--kubeconfig PATH] [--context NAME]
xp-clean plan [--config PATH] [--policy-namespace NS] [--mode …] [--scope …] [--fail-on-blocked] [-o json|yaml]
xp-clean debug trace --name ns/name|--name name -n ns --group G --kind K [--version V] [-o text|json|yaml]
xp-clean manager [--config PATH] [--policy-namespace NS] [--apply] [--mode roots|trace] [--wait]
xp-clean version [-o short|json|yaml]
xp-clean completion bash|zsh|fish|powershell
```

`run --once` is the day-to-day command (dry-run plan by default). `plan` is a deprecated machine-output wrapper. `debug trace` inspects one tree (`-n` only there). Logging uses zap/logr end-to-end, but interactive CLI commands are quiet by default (error-only on stderr). Use `-v=1` / `--log-level=info` for detail (`manager` defaults to info). Encoder flags: `--zap-devel`, `--zap-encoder`. Machine output (`plan`, `debug trace -o json|yaml`, audit JSON on `run`) stays on stdout.

Exit codes: `0` success; `1` runtime failure; `2` usage/validation; `3` blocked plans with `--fail-on-blocked`.

## Run a dry-run locally

Requires a kubeconfig that can list XRDs and the XR GVKs your selectors match. For local file use, pass **`--config` explicitly**. `run` defaults to a dry-run plan:

```bash
go run ./cmd/xp-clean run --once \
  --config config/samples/cleanuppolicy.yaml \
  --kubeconfig "$HOME/.kube/config"
```

```bash
go run ./cmd/xp-clean plan --config config/samples/cleanuppolicy.yaml -o json
```

```bash
go run ./cmd/xp-clean debug trace \
  --name test-db -n xp-test-1 \
  --group db.dev.example.org \
  --kind XDB
```

Live deletes require `--apply` **and** policy `dryRun: false`. Deletes use foreground propagation; add `--wait` to poll until gone. Prefer the default dry-run roots plan until stacks matched / would delete looks correct. Xp-clean never strips Crossplane finalizers.

## In-cluster (Helm)

Prefer the published chart on GHCR (OCI). It installs into the dedicated **`xp-clean`** namespace with ServiceAccount, lab-oriented ClusterRole/Binding (no claim RBAC), a Role for policy ConfigMaps, a sample labelled policy ConfigMap, and a **CronJob** running `xp-clean run --once` by default. Dry-run defaults to **true**. The optional `manager` Deployment is off unless you set `manager.enabled=true`.

Chart `--version` is SemVer **without** a leading `v`; image tags **keep** `v`:

```bash
helm upgrade --install xp-clean oci://ghcr.io/kanzifucius/charts/xp-clean \
  --version 0.1.0-rc.1 \
  --namespace xp-clean \
  --create-namespace
```

From a checkout: `helm upgrade --install xp-clean ./deploy/helm/xp-clean -n xp-clean --create-namespace --set image.tag=v0.1.0-rc.1`. The image and chart packages are **public**, so no `imagePullSecret` is required for the defaults. Hand-rolled manifests remain in [`config/rbac/role.yaml`](config/rbac/role.yaml) and [`config/samples/`](config/samples/) if you are not using Helm. See [Install](https://kanzifucius.github.io/xp-clean/latest/install/) for values and version notes.

## Safety notes

- Default scope deletes **instances only**.
- Live deletes use **foreground** propagation. Polling after delete is **opt-in** (`deletion.wait` / `--wait`); when enabled, `deletion.waitTimeout` applies and stuck finalizers are audited. Crossplane finalizers are **never** stripped.
- Default cleanup mode is **roots** (highest-parent XRs); full composition traces are opt-in (`deletion.mode: trace` / `--mode=trace`).
- With `scope: all`, traces attach CompositionRevision → Composition → XRD; delete order matches that stack. Definition deletes require **zero remaining XRs** for the XRD, and XRD also needs `allowXRDDeletion: true`.
- `xp-clean.io/keep=true` skips the whole trace; paused resources are skipped unless `deletePaused: true`.
- Namespace alone never implies eligibility — use policy labels and/or globs.
- Eligibility never comes from `-n` on `run`/`plan` (`-n` is for `debug trace` only).

## Licence

Licensed under the [Apache License 2.0](LICENSE).

## Community

- [Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Security](SECURITY.md)
