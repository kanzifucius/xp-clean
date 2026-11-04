# Notifications

Chat summaries sit **beside** the CLI summary / audit JSON (when enabled) and metrics. They wrap each `RunOnce` (started / finished / failed), use a short HTTP deadline, and **never** change cleanup results or exit codes.

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

## Teams card content

Finished and dry-run finished cards keep the FactSet counts and add an object list:

| Kind | Heading | Rows |
|------|---------|------|
| Finished (live) | Deleted | Successfully deleted objects + planner reason |
| Finished (dry-run) | Would delete | Planned delete steps + planner reason |
| Failed | Planned / attempted | Plan steps (or audit delete lines) + reason when available |
| Started | — | Lean card; no object list |

Each row is `Kind namespace/name — reason` (cluster-scoped objects omit the namespace). Reasons are copied from the planner / audit (for example `instance_root`); the notify layer does not invent eligibility policy.

**Truncation:** at most **15** objects are included on the Event / card (`notify.MaxObjectsInEvent`) to stay under Teams Adaptive Card payload limits. When more targets exist, the card appends `… and N more (see Open logs)`. Full per-object detail remains in the `run` summary or stdout audit JSON (`-v` / `manager`).

Slack sinks share the same `Event.Objects` fields when implemented; MVP Slack remains unimplemented and counts-oriented.

## Failure isolation

Notify failures are logged and ignored for exit codes. Missing webhook config is a noop. Partial multi-sink failure does not fail cleanup. Audit JSON remains authoritative for what was deleted.

Sequence diagram: [Teams sequence](../architecture/sequence.md).
