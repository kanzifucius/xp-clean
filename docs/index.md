---
search:
  boost: 2
---

# Xp-clean

Xp-clean discovers and cleans up **Crossplane v2** composition trees — test GVKs and leftover instances left behind in shared clusters.

It targets **v2-native** stacks only (**Namespaced** and modern **Cluster** XRs). **LegacyCluster claims are not supported**: there is no claim discovery or deletion path.

## What it does

1. **Discover** CompositeResourceDefinitions and build parent→child **traces** (XR → `spec.crossplane.resourceRefs` → children; nested XRs supported).
2. **Select** eligible stacks with **labels and/or glob patterns** (AND when both are set). Namespace alone never implies eligibility.
3. **Plan** deletions under configurable `deletion.scope`: `instances-only` (default) or `all`.
4. **Execute** safely — dry-run by default, budgets, skip paused resources, never strip Crossplane finalizers.
5. **Notify** (optional) via Microsoft Teams Workflows on run start / finish / failure.

## Documentation map

| Page | Contents |
|------|----------|
| [Quickstart](quickstart.md) | Local dry-run and first ConfigMap |
| [CLI](cli.md) | Commands, flags, exit codes |
| [ConfigMap policy](configmap-policy.md) | In-cluster multi-tenant policy |
| [Eligibility and cleanup](eligibility-cleanup.md) | Matching, scope, safety gates |
| [Notifications](notifications.md) | Teams Workflows hooks |
| [Install and releases](install.md) | Archives, private GHCR, SemVer tags |
| [Architecture](architecture.md) | MVP scope vs deferred work |
| [Contributing](contributing.md) | Repo link and how to help |

## Site features

- **Search** — Material’s built-in client-side search (suggestions, highlighting, shareable query URLs). Use the search box in the header.
- **Versions** — published with [mike](https://github.com/jimporter/mike). Use the version selector in the header.

| Version / alias | Meaning |
|-----------------|---------|
| `latest` | Docs for the newest **non-prerelease** SemVer tag (default after the first release) |
| `0.x.y` | Immutable docs for that release tag |
| `dev` | Docs from the tip of `main` (may change between releases) |

Expected URL: [https://kanzifucius.github.io/xp-clean/](https://kanzifucius.github.io/xp-clean/).
