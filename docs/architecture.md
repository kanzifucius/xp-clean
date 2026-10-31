# Architecture

The locked architecture plan for Xp-clean lives in the project store:

- Cursor Project: `docs/xp-clean-architecture-plan.md`

This repository implements the MVP product slice plus the Cobra / Go layout migration from plan §8 and §11:

- Dynamic XRD discovery + instance trace build (no claims)
- Eligibility via labels and/or globs + TTL (`pkg/match`)
- `deletion.scope` with default `instances-only`
- Dry-run planner + optional deletes
- File/ConfigMap policy; manager schedule stub; audit JSON + metrics stubs
- Cobra CLI: `run` / `plan` / `trace` / `manager` / `version` / `completion`
- Layout: thin `cmd/xp-clean`, `internal/cli` helpers, domain in `pkg/`

Deferred: CleanupPolicy CRD + controller-runtime manager, Prometheus exporters, Helm chart, RBAC generator, kubectl plugin alias, multi-cluster.
