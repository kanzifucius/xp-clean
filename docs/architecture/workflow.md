# Cleanup workflow

One `run --once` pass: load policy → discover → roots (or opt-in full traces) → plan → execute only with `--apply` and policy `dryRun: false` (foreground Delete; optional `--wait`) → audit → notify finished, with AroundRun started wrap.

<div class="archify-embed" markdown="0">
<iframe src="../cleanup-run-once.html" title="Xp-clean cleanup run-once workflow" loading="lazy" referrerpolicy="no-referrer"></iframe>
</div>

<p class="archify-caption">
  End-to-end cleanup pass for a single invocation.
  <a href="../cleanup-run-once.html">Open standalone HTML</a>.
</p>

Operational safety gates for this flow are documented under [Cleanup](../operations/cleanup.md).
