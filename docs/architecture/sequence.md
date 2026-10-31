# Teams sequence

Sequence for `run --once --dry-run=true`: ConfigMap load, Teams `started` POST, RunOnce discover/trace, dry-run-delete audit (no Delete), Teams `finished` POST.

<div class="archify-embed" markdown="0">
<iframe src="../run-once-teams.html" title="Xp-clean run-once with Teams notifications sequence" loading="lazy" referrerpolicy="no-referrer"></iframe>
</div>

<p class="archify-caption">
  Planner dry-run with Microsoft Teams Workflows webhook POSTs around the run.
  <a href="../run-once-teams.html">Open standalone HTML</a>.
</p>

Sink configuration and failure isolation: [Notifications](../operations/notifications.md).
