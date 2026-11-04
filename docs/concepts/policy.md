# Policy and eligibility

In-cluster policy uses **ConfigMaps** in the dedicated **`xp-clean`** namespace. A future `CleanupPolicy` CRD may add richer status UX — ConfigMaps remain the primary surface and are not demoted.

## ConfigMap discovery

| Requirement | Value |
|-------------|--------|
| Namespace | Dedicated `xp-clean` by default (`--policy-namespace` / `XP_CLEAN_POLICY_NAMESPACE`) |
| Label | `xp-clean.io/policy=true` |
| Data key | `policy.yaml` |

Multiple labelled ConfigMaps may coexist for **per-team multi-tenancy** (different TTLs / selectors per team).

Samples in the repository:

- [`config/samples/configmap.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap.yaml) — day-1 team A (labels + TTL + roots)
- [`config/samples/cleanuppolicy.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/cleanuppolicy.yaml) — file form for `--config` only
- [`config/samples/configmap-team-b.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap-team-b.yaml) — advanced multi-tenant globs

### File override

Pass **`--config PATH`** (or `XP_CLEAN_CONFIG`) for local or mounted file use. There is **no** implicit repo-relative sample path.

## Day-1 essentials

Start with these knobs:

| Concern | What to set |
|---------|-------------|
| **Eligibility** | `selectors.requireLabels` (for example `xp-clean.io/eligible: "true"`) |
| **TTL** | `ttl.respectResourceTTL` + `ttl.defaultTTL` |
| **Dry-run** | `dryRun: true` until you are ready; live also needs CLI `--apply` |
| **Mode** | `deletion.mode: roots` (default) — highest-parent XRs only |
| **Scope** | `deletion.scope: instances-only` (default; safer) |

```yaml
name: team-a
dryRun: true
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
ttl:
  respectResourceTTL: true
  defaultTTL: 168h
deletion:
  mode: roots
  scope: instances-only
```

!!! important "Namespace alone is not enough"
    Namespace-implies-eligibility is **not** the model. Eligibility never comes from `-n` / `--namespace` on `run` or `plan` (that flag is rejected there). Use policy labels and/or globs only. `-n` is valid on `debug trace` only.

TTL fields gate when an eligible stack is old enough to delete.

## Cleanup modes

| `deletion.mode` / `--mode` | Behaviour |
|----------------------------|-----------|
| `roots` (**default**) | Discover + match + highest parents. An XR is a root only if it has **no controller ownerReference whose GroupKind is an XR kind** (from the cluster XRD set), even when that parent XR was filtered out by selectors / keep / TTL / paused. Dry-run lists those stacks. Live deletes the root XR (foreground propagation); Crossplane GC cleans dependents. No full composition tree walk |
| `trace` | Opt-in. Builds parent→child traces for detailed plans, nested keep/paused gates, orphans, and richer execute |

## Delete semantics

- Live deletes always use Kubernetes **`DeletionPropagation=Foreground`**.
- **Waiting** after delete is **opt-in**: `deletion.wait: true` or CLI `--wait`. When wait is on, honour `deletion.waitTimeout` and audit stuck finalizers. When wait is off (default), issue the foreground Delete and continue.
- Xp-clean **never** strips Crossplane finalizers.

## Deletion scope

| `deletion.scope` | Behaviour |
|------------------|------------|
| `instances-only` (**default**) | Safer — root XRs (and composed resources via GC / foreground) |
| `all` | Advanced — instances first, then definitions (see below) |

## Advanced

Still supported in the schema; keep them out of first-run samples unless you need them.

### Full glob AND matrix

Matching uses **labels and/or glob patterns**. When both are set, an object must satisfy **all non-empty constraint groups (AND)**.

| Mechanism | Examples |
|-----------|----------|
| Labels | `requireLabels: { xp-clean.io/eligible: "true" }` |
| Namespace globs | `team-*-dev`, `xp-test-*` |
| XRD / XR name globs | `*.dev.example.org`, `test-*` |
| GVK globs | **`group/version`** patterns (kind **not** required), e.g. `*.dev.example.org/*` |
| Deny list | `namespaceDeny: [kube-system, crossplane-system]` |

Day-1 ConfigMap samples stay **label-only** (no `globs.gvks`). To inject GVK globs without editing the policy document, pass repeatable CLI **`--gvk-glob`** on `run` / `plan` / `manager`. When the flag is set it **replaces** `selectors.globs.gvks` entirely; when omitted, the policy value (including none) is used. See [CLI](../cli.md).

### `mode: trace` and `scope: all`

Prefer `deletion.mode: trace` when you need nested keep/paused visibility, orphan reporting detail, or definition-layer planning. `deletion.scope: all` deletes instances first, then definitions (CompositionRevision → Composition → XRD) that have zero remaining instances and match selectors. XRD also needs `allowXRDDeletion: true`.

### Orphan modes, budgets, wait, notify sinks

| Field | Notes |
|-------|--------|
| `deletion.wait` | Opt-in poll-until-gone (default `false`); override with `--wait` |
| `deletion.waitTimeout` | Used only when wait is on (default `30m`) |
| `deletion.orphanMode` | MVP `report` surfaces orphans without inventing parallel ownership |
| `deletion.maxTracesPerRun` / `maxDeletesPerRun` | Per-run budgets |
| `deletion.deletePaused` | Default `false` — skip `crossplane.io/paused` |
| `ttl.applyDefaultTTLIfMissing` | Apply `defaultTTL` when the resource has no TTL annotation |
| `observability.notifications` | Multi-sink schema (Teams MVP); see [Notifications](../operations/notifications.md) |
| `schedule` / `timezone` | Used by `manager` interval stub / future cron |

Example shape with advanced fields filled in:

```yaml
name: team-a
schedule: "0 */6 * * *"
dryRun: true
timezone: Europe/London
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    namespaces: ["team-a-*", "xp-test-*"]
    xrdNames: ["*.dev.example.org"]
    xrNames: ["test-*"]
    gvks: ["*.dev.example.org/*"]   # group/version; kind not required
  namespaceDeny: ["kube-system", "crossplane-system"]
ttl:
  respectResourceTTL: true
  defaultTTL: 168h
  applyDefaultTTLIfMissing: false
deletion:
  mode: trace                       # or roots (default)
  scope: instances-only             # or all
  wait: false
  waitTimeout: 30m
  maxTracesPerRun: 20
  maxDeletesPerRun: 100
  orphanMode: report
  deletePaused: false
  allowXRDDeletion: false
observability:
  verbosity: normal
  notifications:
    enabled: false
    # … see Operations → Notifications
```

Audit events go to **stdout** JSON only for now — no audit ConfigMap or object-storage retention sink.

## RBAC note

Lab / backend installs may use broad reader/deleter ClusterRoles for now; tighten later. Prefer installing into the dedicated `xp-clean` namespace (Helm default).

Next: [Cleanup operations](../operations/cleanup.md) for safety gates and the run flow, or [Architecture](../architecture/index.md) for diagrams of how the runner is wired.
