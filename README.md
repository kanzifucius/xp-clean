# xp-clean

Scheduled discovery and cleanup of Crossplane composition trees (test GVKs and leftover instances).

Xp-clean targets **Crossplane v2-native** stacks only (**Namespaced** / modern **Cluster** XRs). **LegacyCluster claims are not supported** — there is no claim discovery or deletion path.

## Features (MVP)

- Discover CompositeResourceDefinitions and build parent→child **traces** (XR → `spec.crossplane.resourceRefs` → children; nested XRs supported).
- Match eligibility with **labels and/or glob patterns** (AND when both are set).
- Plan deletions with configurable **`deletion.scope`**: `instances-only` (default) or `all`.
- **Dry-run by default** — builds an ordered delete plan and emits JSON audit lines without deleting.
- CLI: `run --once`, `plan`, `trace`, and a thin `manager` schedule stub.

## Build

```bash
go build -o bin/xp-clean ./cmd/xp-clean
go test ./...
```

## Configuration

Policy file (ConfigMap substitute for a future `CleanupPolicy` CRD): see [`config/samples/cleanuppolicy.yaml`](config/samples/cleanuppolicy.yaml).

Important fields:

| Field | Meaning |
|-------|---------|
| `selectors.requireLabels` | Label constraints (e.g. `xp-clean.io/eligible=true`) |
| `selectors.globs` | Name / namespace / GVK globs (`filepath.Match`; `*` does not cross `/`) |
| `deletion.scope` | `instances-only` (default) or `all` |
| `dryRun` | Default `true` |

Empty label **and** empty glob selectors are rejected — namespace alone is never enough.

## Run a dry-run locally

Requires a kubeconfig that can list XRDs and the XR GVKs your selectors match (e.g. a kind / k3d cluster with Crossplane).

```bash
# From the repo root
go run ./cmd/xp-clean run --once \
  --config config/samples/cleanuppolicy.yaml \
  --dry-run=true \
  --kubeconfig "$HOME/.kube/config"
```

Or print plans as JSON:

```bash
go run ./cmd/xp-clean plan --config config/samples/cleanuppolicy.yaml
```

Trace one XR (no delete):

```bash
go run ./cmd/xp-clean trace \
  --name xp-test-1/test-db \
  --group db.dev.example.org \
  --kind XDB
```

Override scope (still dry-run unless you also set dry-run false in config/flags):

```bash
go run ./cmd/xp-clean run --once --scope=all --dry-run=true
```

Live deletes require `dryRun: false` in the policy **and** `--dry-run=false`. Prefer dry-run until the plan looks correct. Xp-clean never strips Crossplane finalizers.

## In-cluster sketch

1. Build/push an image containing `xp-clean`.
2. Deploy a ServiceAccount + ClusterRole (read XRDs/Compositions; get/list/watch/delete matched instance GVKs — see [`config/rbac/role.yaml`](config/rbac/role.yaml)).
3. Mount the policy as a ConfigMap file at `/config/policy.yaml`.
4. Run either:
   - **Job / CronJob:** `xp-clean run --once --config /config/policy.yaml`
   - **Deployment:** `xp-clean manager --config /config/policy.yaml` (MVP uses a simple interval stub derived from `schedule`)

Leader election and a full `CleanupPolicy` CRD reconciler are deferred past this MVP.

## Safety notes

- Default scope deletes **instances only** (XR roots; children follow via ownerReferences / Crossplane GC).
- With `scope: all`, definition order is CompositionRevision → Composition → XRD; XRD deletion also requires `allowXRDDeletion: true`.
- `xp-clean.io/keep=true` skips the whole trace; paused resources are skipped unless `deletePaused: true`.
- Do not point broad globs at production without a dry-run first.

## Layout

See the locked architecture plan (linked from [`docs/architecture.md`](docs/architecture.md)) for package responsibilities: `discover`, `trace`, `match`, `cleanup`, schedule/config, and observability stubs.

## Licence

Apache-2.0 (or as stated by the repository owner).
