# Cleanup

How a planned run stays safe once eligibility has selected stacks. Matching rules themselves are covered in [Policy and eligibility](../concepts/policy.md).

## Safety gates

- **Dry-run by default** — `run --once` always plans first; live deletes need policy `dryRun: false` **and** `--apply` (compatibility: `--dry-run=false`).
- **Roots by default** — `deletion.mode: roots` plans highest-parent XRs only; opt in to full trees with `mode: trace` / `--mode=trace`.
- **Foreground deletes** — live Deletes always use `DeletionPropagation=Foreground`.
- **Wait is opt-in** — default is no poll; `--wait` / `deletion.wait: true` honours `waitTimeout` and stuck-finalizer audit.
- **Paused resources** — skipped by default (`crossplane.io/paused`) unless `deletePaused: true`.
- **Keep label** — `xp-clean.io/keep=true` skips the whole stack.
- **Budgets** — advanced: `maxTracesPerRun` / `maxDeletesPerRun`.
- **Finalizers** — Xp-clean never strips Crossplane finalizers; cloud objects are deleted by Crossplane’s own controllers.
- **Claims / LegacyCluster** — out of scope; XR-only / v2-native path.
- **Orphans** — advanced: `orphanMode: report` (MVP) surfaces orphans without inventing parallel ownership.

## Run flow (summary)

```text
discover XRDs / XRs
  → match labels/globs + TTL
  → highest parents (roots mode) OR build full traces (trace mode)
  → plan deletions (scope + gates)
  → execute only with --apply and policy dryRun: false
       (foreground Delete; optional --wait poll)
  → audit JSON on stdout (+ optional Teams notify)
```

Visual walkthrough: [Cleanup workflow](../architecture/workflow.md).

## Exit behaviour

Use `--fail-on-blocked` when blocked plans should fail the Job (exit **3**). Other exit codes are listed in [CLI](../cli.md).

## Testing cleanup safely

- `make test` runs unit tests (including `pkg/discover`) and the in-memory e2e pipeline under `test/e2e` (dry-run, instances-only delete, keep/paused, exit 3).
- `make e2e` runs the same scenarios against kind + Crossplane with XR→ConfigMap/Deployment fixtures (no cloud providers). Local-first — see the repository [`test/e2e/README.md`](https://github.com/kanzifucius/xp-clean/blob/main/test/e2e/README.md).
