# Architecture

Xp-clean is a Go CLI (and future in-cluster controller) for scheduled discovery and cleanup of Crossplane **v2-native** composition trees.

## MVP implemented in this repository

- Dynamic XRD discovery + instance trace build (**no claims**)
- Eligibility via labels and/or globs + TTL (`pkg/match`); GVK globs are `group/version`
- `deletion.scope` with default `instances-only`
- Boolean dry-run planner + optional deletes; skip paused by default
- File / labelled ConfigMap policy (multi-tenant); explicit `--config` for file override
- Cobra CLI: `run` / `plan` / `trace` / `manager` / `version` / `completion`
- Layout: thin `cmd/xp-clean`, `internal/cli` helpers, domain in `pkg/`
- Optional Teams Workflows notification hooks around `RunOnce`
- GoReleaser + GHCR on SemVer tags (published package currently public; preference remains private — see [Install](../install.md#maintainer-ghcr-visibility)); CI with golangci-lint

## Deferred

CleanupPolicy CRD + controller-runtime manager, Prometheus exporters, Helm chart, RBAC generator, kubectl plugin alias, multi-cluster, Slack sink implementation, audit retention sinks, SPDX licence packaging.

## Diagrams

Interactive **Archify** HTML (showcase quality) is embedded below each caption page. Open the standalone file if you prefer a full browser tab.

| Diagram | Page | Standalone HTML |
|---------|------|-----------------|
| Runtime architecture | [Runtime diagram](runtime.md) | [xp-clean-runtime.html](xp-clean-runtime.html) |
| Cleanup run-once workflow | [Cleanup workflow](workflow.md) | [cleanup-run-once.html](cleanup-run-once.html) |
| `run --once` + Teams sequence | [Teams sequence](sequence.md) | [run-once-teams.html](run-once-teams.html) |

Regenerated from repository-backed Archify candidates under `.archify/` (not committed). Pin evidence with `--repo-root` and `--quality showcase`.

## Design plans

Longer design notes are maintained alongside the Cursor project for this repository (architecture, release plumbing, notification hooks). Product behaviour summarised in these docs should match those locked decisions; when in doubt, prefer the samples under `config/samples/` and the CLI help text in the current release.
