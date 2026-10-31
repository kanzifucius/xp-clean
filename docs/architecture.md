# Architecture

The locked architecture plan for Xp-clean lives in the project store:

- Cursor Project: `docs/xp-clean-architecture-plan.md`

This MVP implements the plan’s MVP slice:

- Dynamic XRD discovery + instance trace build (no claims)
- Eligibility via labels and/or globs + TTL
- `deletion.scope` with default `instances-only`
- Dry-run planner + optional deletes
- File/ConfigMap policy; manager schedule stub; audit JSON + metrics stubs
- CLI `run --once` / `plan` / `trace` / `manager`

Deferred (v1+): CleanupPolicy CRD, CompositionTrace status, full Prometheus exporters, Helm chart, RBAC generator, multi-cluster.
