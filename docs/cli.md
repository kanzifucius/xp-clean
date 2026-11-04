# CLI

Standalone Cobra binary `xp-clean`. kubectl plugin documentation is deferred — use the standalone CLI for now.

## Commands

```text
xp-clean run --once [--config PATH] [--policy-namespace NS] [--apply] [--mode roots|trace] [--wait] [--dry-run=true|false] [--scope instances-only|all] [--gvk-glob G/V]… [--fail-on-blocked] [--notify] [--notify-on-dry-run] [--kubeconfig PATH] [--context NAME]
xp-clean plan [--config PATH] [--policy-namespace NS] [--mode …] [--scope …] [--gvk-glob G/V]… [--fail-on-blocked] [-o json|yaml]
xp-clean debug trace --name ns/name|--name name -n ns --group G --kind K [--version V] [-o text|json|yaml]
xp-clean manager [--config PATH] [--policy-namespace NS] [--apply] [--mode roots|trace] [--wait] [--gvk-glob G/V]…
xp-clean skill install [--project] [--force] [-a cursor]
xp-clean version [-o short|json|yaml]
xp-clean completion bash|zsh|fish|powershell
```

| Command | Role |
|---------|------|
| `run --once` | **Day-to-day.** Discover → plan → optional execute. Defaults to a **dry-run plan** of **roots** (highest-parent stacks matched / would delete) |
| `plan` | Deprecated thin wrapper: machine-readable DeletePlans (`-o json\|yaml`), always dry-run |
| `debug trace` | Print one composition tree (debug only; `-n` valid here) |
| `manager` | Long-running schedule stub (same `--apply` / dry-run / mode / wait gates as `run`) |
| `skill install` | Install the embedded Cursor skill (`~/.cursor/skills/xp-clean/` by default; `--project` for repo-local) |
| `version` / `completion` | Build metadata and shell completion |

Root `xp-clean trace` remains as a **hidden** compatibility alias of `debug trace`.

## Flags worth knowing

| Flag / env | Notes |
|------------|--------|
| `--config` / `XP_CLEAN_CONFIG` | **Required** for file override — no implicit sample path |
| `--policy-namespace` / `XP_CLEAN_POLICY_NAMESPACE` | Namespace for labelled ConfigMaps (default `xp-clean`) |
| `--apply` | Happy-path **live** switch. Still requires policy `dryRun: false`. Without `--apply`, `run` always plans only |
| `--mode` / `XP_CLEAN_MODE` | `roots` (default) or `trace`. Roots-only plans highest-parent XRs; `trace` builds full parent→child trees |
| `--wait` / `XP_CLEAN_WAIT` | Opt-in: poll until each delete is gone (honours `deletion.waitTimeout`). **Default is no wait** |
| `--dry-run` / `XP_CLEAN_DRY_RUN` | Compatibility boolean planner dry-run (not kubectl API-server dry-run). `--dry-run=true` forces a plan; `--dry-run=false` allows live only when policy `dryRun` is false (prefer `--apply`) |
| `--scope` / `XP_CLEAN_SCOPE` | `instances-only` (default) or `all` |
| `--gvk-glob` | Repeatable `group/version` glob (same dialect as `selectors.globs.gvks`). When set, **replaces** policy GVK globs entirely (no merge). When omitted, policy globs (including none) are unchanged. Labels / TTL / other globs stay from policy |
| `--fail-on-blocked` | Exit **3** when the run completed with blocked plans |
| `--notify` / `XP_CLEAN_NOTIFY` | Override `notifications.enabled` |
| `--notify-on-dry-run` / `XP_CLEAN_NOTIFY_ON_DRY_RUN` | Override `notifications.onDryRun` |
| `-n` / `--namespace` | Valid on **`debug trace` only**. Rejected on `run` / `plan` (usage / exit 2) |
| `-v` / `--v` | Opt-in log verbosity: `0` error-only (default), `1` info, `2+` debug |
| `--log-level` | Zap level on stderr (`debug\|info\|warn\|error`); default **`error`** for interactive CLI. Alias: `--zap-log-level` |

Logging stays on **zap/logr** end-to-end, but interactive commands (`run`, `plan`, `debug trace`, `version`) are **quiet by default**: only command results on stdout and real failures on stderr. Pass `-v=1` (or `--log-level=info`) to restore detail. `manager` defaults to **info**. Client-go / discovery warning noise is muted unless verbose. Encoder flags: `--zap-devel`, `--zap-encoder`.

### `run` output

Default `run --once` (roots + instances-only) prints a compact human summary on **stdout**:

```text
policy=<name> dryRun=<bool> mode=roots scope=instances-only
would delete N
  <Kind> <namespace>/<name> — <reason>
  …
```

Each object line includes the planner reason (for example `instance_root`). The same `Kind namespace/name — reason` shape is used on Teams Adaptive Cards.

No per-object audit JSON, no `trace …` dumps, and no `stack … steps=1` / numbered delete lines for single-step root plans. Restore richer plan detail and stdout audit JSON with `-v=1` (or `--log-level=info`). `mode=trace`, `scope=all`, and multi-step plans keep detailed stack listings without requiring `-v`. Machine-readable DeletePlans remain on `plan -o json|yaml`. Scheduled `manager` runs still emit audit JSON on stdout.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Runtime failure |
| `2` | Usage / validation (including `-n` on `run`/`plan`) |
| `3` | Blocked plans with `--fail-on-blocked` |

## Precedence

Flags > env > policy document > built-in defaults. No Viper. Webhook URLs come only from env vars named by each sink (default `XP_CLEAN_TEAMS_WEBHOOK_URL`).

Live deletes are **two-key**: CLI `--apply` (or compatibility `--dry-run=false`) **and** policy `dryRun: false`. Flags never unilaterally force live past a policy that still has `dryRun: true`.

Live deletes always use Kubernetes **`DeletionPropagation=Foreground`**. Waiting for the object to disappear is separate and **off by default**.

## Migration notes

| Old habit | Prefer now |
|-----------|------------|
| `xp-clean plan` for interactive dry-run | `xp-clean run --once` |
| `xp-clean plan -o json` in scripts | Still works (deprecated wrapper); or parse `run` summary / audit |
| `xp-clean trace …` | `xp-clean debug trace …` |
| `--dry-run=false` for live | `--apply` (and keep policy `dryRun: false`) |
| Always walk full composition trees | Default `roots`; opt in with `--mode=trace` / `deletion.mode: trace` |
| Always poll after delete | Opt in with `--wait` / `deletion.wait: true` |
