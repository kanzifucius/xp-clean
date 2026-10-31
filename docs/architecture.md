# Architecture

The locked architecture plan for Xp-clean lives in the project store:

- Cursor Project: `docs/xp-clean-architecture-plan.md`

This repository implements the MVP product slice plus locked CLI / policy decisions:

- Dynamic XRD discovery + instance trace build (**no claims** / LegacyCluster claim paths)
- Eligibility via **labels and/or globs** + TTL (`pkg/match`); GVK globs are **`group/version`**
- `deletion.scope` with default **`instances-only`**; skip paused resources by default
- Boolean planner `--dry-run`; `--fail-on-blocked` → exit **3**
- **ConfigMaps** as primary in-cluster policy (multi-tenant / per-team in the dedicated **`xp-clean`** namespace); explicit **`--config`** file override only (no implicit sample path)
- zap/logr end-to-end; stdout audit JSON only (no audit retention sink)
- Cobra CLI: `run` / `plan` / `trace` / `manager` / `version` / `completion`
- `-n` / `--namespace` rejected on `run`/`plan`; valid on `trace` only
- Layout: thin `cmd/xp-clean`, `internal/cli` helpers, domain in `pkg/`

Deferred (separate work): CleanupPolicy CRD + controller-runtime manager, Prometheus exporters, Helm chart packaging, RBAC generator, GoReleaser/GHCR, Teams/Slack notify, kubectl plugin alias / `kubectl-xpclean` docs.
