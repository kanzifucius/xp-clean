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
- GoReleaser + private GHCR on SemVer tags; CI with golangci-lint

## Deferred

CleanupPolicy CRD + controller-runtime manager, Prometheus exporters, Helm chart, RBAC generator, kubectl plugin alias, multi-cluster, Slack sink implementation, audit retention sinks, SPDX licence packaging.

## Design plans

Longer design notes are maintained alongside the Cursor project for this repository (architecture, release plumbing, notification hooks). Product behaviour summarised in these docs should match those locked decisions; when in doubt, prefer the samples under `config/samples/` and the CLI help text in the current release.
