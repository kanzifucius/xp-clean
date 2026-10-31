# Concepts

How Xp-clean decides **what** is in scope and **how** operators express that as policy.

## Start here

| Topic | Why it matters |
|-------|----------------|
| [Policy and eligibility](policy.md) | ConfigMap discovery, selectors (labels / globs / TTL), deletion scope, and the policy document shape |

## Mental model

1. Policy lives in labelled **ConfigMaps** (or a file via `--config`) — not in a CleanupPolicy CRD yet.
2. Eligibility is **labels and/or globs**, never “everything in this namespace”.
3. Deletion scope defaults to the safer **`instances-only`** path; definitions need an explicit `all` scope plus XRD flags.
4. Dry-run is the default until both policy and CLI ask for live deletes.

For day-to-day run safety, budgets, and Teams hooks, continue in [Operations](../operations/index.md). For command flags, see [CLI](../cli.md).
