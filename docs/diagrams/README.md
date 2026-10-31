# Architecture diagrams

Standalone Archify HTML diagrams for Xp-clean (British English labels; showcase quality). Open any file in a browser — no build step.

| Diagram | File | Shows |
|---------|------|--------|
| Runtime architecture | [xp-clean-runtime.html](xp-clean-runtime.html) | CLI (`run` / `plan` / `trace` / `manager`), `cleanup.Runner`, discover/match/trace, Kubernetes API, ConfigMap policy, Crossplane XR/MR tree, Teams notify sink |
| Cleanup workflow | [cleanup-run-once.html](cleanup-run-once.html) | One cleanup pass: discover → plan → delete/dry-run → audit → notify |
| Sequence | [run-once-teams.html](run-once-teams.html) | `run --once` with planner dry-run and Teams Workflows webhook POSTs |

Regenerated from repository-backed Archify candidates under `.archify/` (not committed). Pin evidence with `--repo-root` and `--quality showcase`.
