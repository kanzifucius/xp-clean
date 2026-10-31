# CLI

Standalone Cobra binary `xp-clean`. kubectl plugin documentation is deferred — use the standalone CLI for now.

## Commands

```text
xp-clean run --once [--config PATH] [--policy-namespace NS] [--dry-run=true|false] [--scope instances-only|all] [--fail-on-blocked] [--notify] [--notify-on-dry-run] [--kubeconfig PATH] [--context NAME]
xp-clean plan [--config PATH] [--policy-namespace NS] [--scope …] [--fail-on-blocked] [-o json|yaml]
xp-clean trace --name ns/name|--name name -n ns --group G --kind K [--version V] [-o text|json|yaml]
xp-clean manager [--config PATH] [--policy-namespace NS]
xp-clean version [-o short|json|yaml]
xp-clean completion bash|zsh|fish|powershell
```

| Command | Role |
|---------|------|
| `run --once` | Discover → plan → optional execute (CronJob-friendly) |
| `plan` | Planner output only (always dry-run semantics for deletes) |
| `trace` | Print one composition tree |
| `manager` | Long-running schedule stub |
| `version` / `completion` | Build metadata and shell completion |

## Flags worth knowing

| Flag / env | Notes |
|------------|--------|
| `--config` / `XP_CLEAN_CONFIG` | **Required** for file override — no implicit sample path |
| `--policy-namespace` / `XP_CLEAN_POLICY_NAMESPACE` | Namespace for labelled ConfigMaps (default `xp-clean`) |
| `--dry-run` / `XP_CLEAN_DRY_RUN` | Boolean planner dry-run (not kubectl API-server dry-run). Unset keeps policy value (default `true`) |
| `--scope` / `XP_CLEAN_SCOPE` | `instances-only` (default) or `all` |
| `--fail-on-blocked` | Exit **3** when the run completed with blocked plans |
| `--notify` / `XP_CLEAN_NOTIFY` | Override `notifications.enabled` |
| `--notify-on-dry-run` / `XP_CLEAN_NOTIFY_ON_DRY_RUN` | Override `notifications.onDryRun` |
| `-n` / `--namespace` | Valid on **`trace` only**. Rejected on `run` / `plan` (usage / exit 2) |

Logging uses **zap/logr** on stderr (`--zap-devel`, `--zap-log-level`, `--zap-encoder`). Machine output (`plan`, `trace -o json|yaml`, audit JSON on `run`) stays on **stdout**.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Runtime failure |
| `2` | Usage / validation (including `-n` on `run`/`plan`) |
| `3` | Blocked plans with `--fail-on-blocked` |

## Precedence

Flags > env > policy document > built-in defaults. No Viper. Webhook URLs come only from env vars named by each sink (default `XP_CLEAN_TEAMS_WEBHOOK_URL`).
