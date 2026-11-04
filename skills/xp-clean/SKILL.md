---
name: xp-clean
description: Plan and clean Crossplane v2 composition leftovers with the xp-clean CLI. Use when the user asks to dry-run, plan, or apply xp-clean cleanup; never invent broad GVK lists.
---

# xp-clean

Help the human run **xp-clean** safely against Crossplane v2-native stacks (XR-only; no LegacyCluster claims).

## Hard rules

1. **Always dry-run / plan first.** Prefer `xp-clean run --once` (default is a dry-run plan). Do **not** pass `--apply` on the first pass.
2. **Never pass `--apply` unless the human explicitly asks** for live deletes. Before applying, confirm the dry-run output (stacks matched / would delete) with them.
3. **Skinny eligibility only.** Match via label `xp-clean.io/eligible=true` and TTL from policy. Do **not** invent broad GVK lists or expand selectors beyond what the human or policy already specifies.
4. Point at **`--config PATH`** whenever using a file policy override (no implicit sample path).

## Defaults to keep

Unless the human opts in otherwise:

| Concern | Default |
|---------|---------|
| Mode | **roots** (highest-parent XRs); `--mode=trace` only when they need full trees |
| Scope | **instances-only**; `--scope=all` is advanced |
| Propagation | Live Deletes use Kubernetes **foreground** propagation (built-in; not a flag) |
| Wait | **No wait** after delete; `--wait` is opt-in |

Interactive CLI output is quiet by default (compact would-delete summary on `run`; no audit JSON). Use `-v` / `--log-level` for audit JSON and full stack/trace detail.

## GVK globs (`--gvk-glob`)

Repeatable flag: `--gvk-glob group/version` (same dialect as policy `selectors.globs.gvks`; kind not required).

- **When set** (one or more times): **replaces** policy/config `selectors.globs.gvks` entirely — no merge.
- **When omitted**: policy GVK globs apply unchanged (including **none** for label-only skinny policies).

Prefer omitting `--gvk-glob` for day-1 label+TTL policies. Only add globs when the human asks to constrain or override GVKs for this run.

```bash
# Label-only skinny policy (typical)
xp-clean run --once --config ./policy.yaml

# Override GVK globs for this invocation only
xp-clean run --once --config ./policy.yaml \
  --gvk-glob '*.dev.example.org/*' \
  --gvk-glob 'apps.example.org/v1'
```

## Typical commands

```bash
# Dry-run plan (default) — always start here
xp-clean run --once --config PATH

# Live deletes — only after explicit human confirmation
xp-clean run --once --config PATH --apply
# (also requires policy dryRun: false)

# Opt-ins
xp-clean run --once --config PATH --mode=trace
xp-clean run --once --config PATH --apply --wait
```

In-cluster policies load from labelled ConfigMaps in `--policy-namespace` (default `xp-clean`) when `--config` is unset.

## Install this skill

```bash
xp-clean skill install            # ~/.cursor/skills/xp-clean/SKILL.md
xp-clean skill install --project  # .cursor/skills/xp-clean/SKILL.md
xp-clean skill install --force    # overwrite existing
```
