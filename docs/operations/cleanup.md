# Cleanup

How a planned run stays safe once eligibility has selected stacks. Matching rules themselves are covered in [Policy and eligibility](../concepts/policy.md).

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

Visual walkthrough: [Cleanup workflow](../architecture/workflow.md).

## Exit behaviour

Use `--fail-on-blocked` when blocked plans should fail the Job (exit **3**). Other exit codes are listed in [CLI](../cli.md).
