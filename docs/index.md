---
search:
  boost: 2
---

# Xp-clean

Xp-clean discovers and cleans up **Crossplane v2** composition trees — test GVKs and leftover instances left behind in shared clusters.

It targets **v2-native** stacks only (**Namespaced** and modern **Cluster** XRs). **LegacyCluster claims are not supported**: there is no claim discovery or deletion path.

## What it does

1. **Discover** CompositeResourceDefinitions and select **highest-parent** XRs (roots mode). Full parent→child **traces** are opt-in (`--mode=trace`).
2. **Select** eligible stacks with **labels** (day-1) and/or **glob patterns** (advanced). Namespace alone never implies eligibility.
3. **Plan** with `xp-clean run --once` — dry-run by default (`deletion.mode: roots`, `deletion.scope: instances-only`).
4. **Execute** only with `--apply` and policy `dryRun: false` (foreground Delete; `--wait` opt-in); skip paused resources; never strip Crossplane finalizers.
5. **Notify** (optional, advanced) via Microsoft Teams Workflows on run start / finish / failure.

## Where to start

<div class="doc-map" markdown="1">

<div markdown="1">

**[Install](install.md)**  
Curl installer, pinned versions, GHCR, and release notes.

</div>

<div markdown="1">

**[Quickstart](quickstart.md)**  
Local dry-run against a sample policy in a few commands.

</div>

<div markdown="1">

**[Concepts](concepts/index.md)**  
ConfigMap policy shape and how eligibility matching works.

</div>

<div markdown="1">

**[Architecture](architecture/index.md)**  
MVP scope plus embedded Archify diagrams of runtime, workflow, and Teams sequence.

</div>

<div markdown="1">

**[CLI](cli.md)**  
Commands, flags, exit codes, and precedence.

</div>

<div markdown="1">

**[Operations](operations/index.md)**  
Cleanup safety gates and Teams notification hooks.

</div>

</div>

Contributing and local docs builds are covered in [Contributing](contributing.md).

## Site features

- **Search** — Material’s built-in client-side search (suggestions, highlighting, shareable query URLs). Use the search box in the header.
- **Versions** — published with [mike](https://github.com/jimporter/mike). Use the version selector in the header.

| Version / alias | Meaning |
|-----------------|---------|
| `latest` | Docs for the newest **non-prerelease** SemVer tag (default after the first release) |
| `0.x.y` | Immutable docs for that release tag |
| `dev` | Docs from the tip of `main` (may change between releases) |

Expected URL: [https://kanzifucius.github.io/xp-clean/](https://kanzifucius.github.io/xp-clean/).
