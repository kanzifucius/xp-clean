# Eligibility and cleanup

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

## Safety gates

- **Dry-run by default** — planner dry-run; live deletes need policy `dryRun: false` **and** `--dry-run=false`.
- **Paused resources** — skipped by default (`crossplane.io/paused`) unless `deletePaused: true`.
- **Keep label** — `xp-clean.io/keep=true` skips the whole trace.
- **Budgets** — `maxTracesPerRun` / `maxDeletesPerRun`.
- **Finalizers** — Xp-clean never strips Crossplane finalizers; cloud objects are deleted by Crossplane’s own controllers.
- **Claims / LegacyCluster** — out of scope; XR-only / v2-native path.
- **Orphans** — `orphanMode: report` (MVP) surfaces orphans without inventing parallel ownership.

## Run flow (summary)

```text
discover XRDs / XRs
  → match labels/globs + TTL
  → build traces (parent→child)
  → plan deletions (scope + gates)
  → execute (unless dry-run)
  → audit JSON on stdout (+ optional Teams notify)
```
