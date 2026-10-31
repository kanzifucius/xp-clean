# ConfigMap policy

In-cluster policy configuration uses **ConfigMaps** in the dedicated **`xp-clean`** namespace. A future `CleanupPolicy` CRD may add richer status UX — ConfigMaps remain the primary surface and are not demoted.

## Discovery rules

| Requirement | Value |
|-------------|--------|
| Namespace | Dedicated `xp-clean` by default (`--policy-namespace` / `XP_CLEAN_POLICY_NAMESPACE`) |
| Label | `xp-clean.io/policy=true` |
| Data key | `policy.yaml` |

Multiple labelled ConfigMaps may coexist for **per-team multi-tenancy** (different TTLs / globs per team).

Samples in the repository:

- [`config/samples/configmap.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap.yaml) — team A
- [`config/samples/configmap-team-b.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/configmap-team-b.yaml) — team B
- [`config/samples/cleanuppolicy.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/config/samples/cleanuppolicy.yaml) — file form for `--config` only

## File override

Pass **`--config PATH`** (or `XP_CLEAN_CONFIG`) for local or mounted file use. There is **no** implicit repo-relative sample path.

## Policy document shape (summary)

```yaml
name: team-a
schedule: "0 */6 * * *"
dryRun: true
timezone: Europe/London
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    namespaces: ["team-a-*", "xp-test-*"]
    xrdNames: ["*.dev.example.org"]
    xrNames: ["test-*"]
    gvks: ["*.dev.example.org/*"]   # group/version; kind not required
  namespaceDeny: ["kube-system", "crossplane-system"]
ttl:
  respectResourceTTL: true
  defaultTTL: 168h
  applyDefaultTTLIfMissing: false
deletion:
  scope: instances-only             # or all
  maxTracesPerRun: 20
  maxDeletesPerRun: 100
  orphanMode: report
  deletePaused: false
  allowXRDDeletion: false
  waitTimeout: 30m
observability:
  verbosity: normal
  notifications:
    enabled: false
    # … see Notifications
```

Audit events go to **stdout** JSON only for now — no audit ConfigMap or object-storage retention sink.

## RBAC note

Lab / backend installs may use broad reader/deleter ClusterRoles for now; tighten later. Prefer installing into the dedicated `xp-clean` namespace (Helm default when the chart lands).
