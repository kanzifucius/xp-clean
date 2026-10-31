# Runtime diagram

How the CLI, `cleanup.Runner`, policy loader, Kubernetes API, Crossplane XR/MR tree, and Teams notify sink relate at runtime.

<div class="archify-embed" markdown="0">
<iframe src="../xp-clean-runtime.html" title="Xp-clean runtime architecture" loading="lazy" referrerpolicy="no-referrer"></iframe>
</div>

<p class="archify-caption">
  Operator → Cobra CLI → <code>cleanup.Runner</code> → discover / match / trace → plan / execute;
  ConfigMap policy; Kubernetes API + XR/MR tree; AroundRun → Teams Workflows; stdout audit.
  <a href="../xp-clean-runtime.html">Open standalone HTML</a>.
</p>

See also [Cleanup workflow](workflow.md) and [Teams sequence](sequence.md).
