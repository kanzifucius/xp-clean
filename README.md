# xp-clean

Scheduled discovery and cleanup of Crossplane composition trees (test GVKs and leftover instances).

Xp-clean targets **Crossplane v2-native** stacks only (**Namespaced** / modern **Cluster** XRs). **LegacyCluster claims are not supported** — there is no claim discovery or deletion path.

## Features (MVP)

- Discover CompositeResourceDefinitions and build parent→child **traces** (XR → `spec.crossplane.resourceRefs` → children; nested XRs supported).
- Match eligibility with **labels and/or glob patterns** (AND when both are set) via `pkg/match` (eligibility in the architecture plan).
- Plan deletions with configurable **`deletion.scope`**: `instances-only` (default) or `all`.
- **Dry-run by default** — planner dry-run (client-side `DeletePlan` only; not kubectl API-server dry-run).
- Cobra CLI: `run`, `plan`, `trace`, `manager`, `version`, `completion`.

## Build

```bash
make test
make build   # writes bin/xp-clean with version ldflags
```

## Layout

Follows typical Go / Kubernetes operator layout (see architecture plan §11):

| Path | Role |
|------|------|
| `cmd/xp-clean/` | Thin Cobra entrypoints (`main`, `root`, subcommands, `options/`) |
| `internal/cli/` | Cobra-agnostic helpers: kube `clientcmd`, logging, exit codes, policy overrides |
| `internal/conf`, `internal/controller`, `internal/version` | Config load, schedule stub, build metadata |
| `pkg/` | Importable domain library (`discover`, `trace`, `match`, `cleanup`, …) — no Cobra |
| `config/` | Samples + RBAC manifests |

Domain stays in `pkg/` so CLI and a future controller share one `cleanup.Runner`.

## Configuration

Policy file (ConfigMap substitute for a future `CleanupPolicy` CRD): [`config/samples/cleanuppolicy.yaml`](config/samples/cleanuppolicy.yaml).

**Precedence:** flags > env (`XP_CLEAN_CONFIG`, `XP_CLEAN_DRY_RUN`, `XP_CLEAN_SCOPE`) > policy document > built-in defaults. No Viper.

| Field / flag | Meaning |
|--------------|---------|
| `selectors.requireLabels` / globs | Labels and/or globs (required non-empty) |
| `deletion.scope` / `--scope` | `instances-only` (default) \| `all` |
| `dryRun` / `--dry-run` | Planner dry-run; unset flag keeps policy value (default true) |
| `--kubeconfig`, `--context`, `-n` | kubectl-style access; `-n` is for `trace` only (rejected on `run`/`plan`) |

## CLI

```text
xp-clean run --once [--config PATH] [--dry-run] [--scope instances-only|all] [--kubeconfig PATH] [--context NAME]
xp-clean plan [--config PATH] [--scope …] [-o json|yaml]
xp-clean trace --name ns/name|--name name -n ns --group G --kind K [--version V] [-o text|json|yaml]
xp-clean manager [--config PATH]
xp-clean version [-o short|json|yaml]
xp-clean completion bash|zsh|fish|powershell
```

Logging uses zap/logr on stderr (`--zap-devel`, `--zap-log-level`, `--zap-encoder`). Machine output (`plan`, `trace -o json|yaml`, audit JSON on `run`) stays on stdout.

## Run a dry-run locally

Requires a kubeconfig that can list XRDs and the XR GVKs your selectors match.

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
2. Deploy a ServiceAccount + ClusterRole — see [`config/rbac/role.yaml`](config/rbac/role.yaml).
3. Mount the policy as a ConfigMap at `/config/policy.yaml`.
4. Run either:
   - **Job / CronJob:** `xp-clean run --once --config /config/policy.yaml`
   - **Deployment:** `xp-clean manager --config /config/policy.yaml` (interval stub; controller-runtime later)

## Safety notes

- Default scope deletes **instances only**.
- With `scope: all`, order is CompositionRevision → Composition → XRD; XRD also needs `allowXRDDeletion: true`.
- `xp-clean.io/keep=true` skips the whole trace; paused resources are skipped unless `deletePaused: true`.
- Namespace alone never implies eligibility.

## Licence

Apache-2.0 (or as stated by the repository owner).
