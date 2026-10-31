# Notifications

Chat summaries sit **beside** stdout audit JSON and metrics. They wrap each `RunOnce` (started / finished / failed), use a short HTTP deadline, and **never** change cleanup results or exit codes.

MVP sink: **Microsoft Teams Workflows**. Slack shares the multi-sink schema but is not implemented yet.

## Decisions (locked)

- Notify on **run start** when notifications are enabled (`started` in `notifyOn`).
- Multi-sink schema from day one; Teams first.
- `onDryRun: true` — dry-run runs still notify when enabled.
- Optional `logURLTemplate` for log deep-links (`{{runId}}` substituted).
- SSRF host allow-list is guidance-only for MVP (https-only client; no redirect following).
- Ships with **ConfigMap / `--config`** now — do not wait for a CleanupPolicy CRD.

## Teams Workflow setup

1. In Teams, create a **Workflow** owned by the **platform team** or a **channel owner** (both are valid; prefer co-owners).
2. Trigger: **When a Teams webhook request is received**.
3. Action: **Post card in a chat or channel**.
4. Copy the Workflow callback URL into a Kubernetes Secret (or local env) as `XP_CLEAN_TEAMS_WEBHOOK_URL`.
5. **Never** commit the URL to git or put it in the ConfigMap policy body.

## Policy snippet

```yaml
observability:
  notifications:
    enabled: true
    notifyOn: [started, finished, failed]
    onDryRun: true
    sinks:
      - type: teams
        webhookURLFromEnv: XP_CLEAN_TEAMS_WEBHOOK_URL
        logURLTemplate: "https://logs.example/search?q={{runId}}"  # optional
```

Mount the Secret as an env var on the Job / CronJob / Deployment. Overrides:

| Flag / env | Effect |
|------------|--------|
| `--notify` / `XP_CLEAN_NOTIFY` | Override `notifications.enabled` |
| `--notify-on-dry-run` / `XP_CLEAN_NOTIFY_ON_DRY_RUN` | Override `onDryRun` |

## Failure isolation

Notify failures are logged and ignored for exit codes. Missing webhook config is a noop. Partial multi-sink failure does not fail cleanup. Audit JSON remains authoritative for what was deleted.

Sequence diagram: [Teams sequence](../architecture/sequence.md).
