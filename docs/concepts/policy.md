# Policy and eligibility

In-cluster policy uses **ConfigMaps** in the dedicated **`xp-clean`** namespace. A future `CleanupPolicy` CRD may add richer status UX — ConfigMaps remain the primary surface and are not demoted.

## ConfigMap discovery

| Requirement | Value |
|-------------|--------|
| Namespace | Dedicated `xp-clean` by default (`--policy-namespace` / `XP_CLEAN_POLICY_NAMESPACE`) |
| Label | `xp-clean.io/policy=true` |
| Data key | `policy.yaml` |

Multiple labelled ConfigMaps may coexist for **per-team multi-tenancy** (different TTLs / globs per team).

Samples in the repository:

- [`config/samples/configmap.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap.yaml) — team A
- [`config/samples/configmap-team-b.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap-team-b.yaml) — team B
- [`config/samples/cleanuppolicy.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/cleanuppolicy.yaml) — file form for `--config` only

### File override

Pass **`--config PATH`** (or `XP_CLEAN_CONFIG`) for local or mounted file use. There is **no** implicit repo-relative sample path.

## Eligibility

Matching uses **labels and/or glob patterns**. Operators may use labels, globs, or both. When both are set, an object must satisfy **all non-empty constraint groups (AND)**.

| Mechanism | Examples |
|-----------|----------|
| Labels | `requireLabels: { xp-clean.io/eligible: "true" }` |
| Namespace globs | `team-*-dev`, `xp-test-*` |
| XRD / XR name globs | `*.dev.example.org`, `test-*` |
| GVK globs | **`group/version`** patterns (kind **not** required), e.g. `*.dev.example.org/*` |
| Deny list | `namespaceDeny: [kube-system, crossplane-system]` |

!!! important "Namespace alone is not enough"
    Namespace-implies-eligibility is **not** the model. Eligibility never comes from `-n` / `--namespace` on `run` or `plan` (that flag is rejected there). Use policy labels and/or globs only.

TTL fields (`respectResourceTTL`, `defaultTTL`, …) gate when an eligible stack is old enough to delete.

## Deletion scope

| `deletion.scope` | Behaviour |
|------------------|------------|
| `instances-only` (**default**) | Safer — XRs and composed resources / MRs |
| `all` | Instances first, then definitions (CompositionRevision → Composition → XRD) that have zero remaining instances and match selectors. XRD also needs `allowXRDDeletion: true` |

## Policy document shape (summary)

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
  scope: instances-only             # or all
  maxTracesPerRun: 20
  maxDeletesPerRun: 100
  orphanMode: report
  deletePaused: false
  allowXRDDeletion: false
  waitTimeout: 30m
observability:
  verbosity: normal
  notifications:
    enabled: false
    # … see Operations → Notifications
```

Audit events go to **stdout** JSON only for now — no audit ConfigMap or object-storage retention sink.

## RBAC note

Lab / backend installs may use broad reader/deleter ClusterRoles for now; tighten later. Prefer installing into the dedicated `xp-clean` namespace (Helm default when the chart lands).

Next: [Cleanup operations](../operations/cleanup.md) for safety gates and the run flow, or [Architecture](../architecture/index.md) for diagrams of how the runner is wired.
