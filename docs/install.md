# Install and releases

Tagged SemVer releases (`v*`) publish release artefacts. There is **no** snapshot publish from `main`.

## Artefacts

1. **GitHub Release archives** — `linux` and `darwin` (`amd64` / `arm64`) plus `checksums.txt` (and archive SBOMs when Syft is available on the runner).
2. **Private GHCR image** — `ghcr.io/kanzifucius/xp-clean` (linux `amd64` / `arm64`).

| Image tag | Meaning |
|-----------|---------|
| `:vX.Y.Z` | Immutable pin (prefer this in clusters) |
| `:X.Y` / `:X` | Floating minor / major on non-prerelease cuts |
| `:latest` | Moved only on **non-prerelease** releases (RCs such as `v0.2.0-rc.1` do not move it) |

GHCR packages stay **private** by preference (even though this GitHub repository is public). Release archives on GitHub are public with the repository.

## CLI on your machine

Install the latest **non-prerelease** binary into `~/.local/bin` (created if missing; the script never runs `sudo` for you):

```bash
curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash
```

Until the first non-prerelease cut, pin the current pre-release explicitly (for example `v0.1.0-rc.1`):

```bash
XP_CLEAN_VERSION=v0.1.0-rc.1 curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash
```

### Pin a version

```bash
XP_CLEAN_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash
# or:
curl -fsSL https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh | bash -s -- --version v0.1.0
```

Use `XP_CLEAN_VERSION` (or `--version`) for any tag — including RCs such as `v0.1.0-rc.1`. Omit it only when you want the latest **non-prerelease**.

### Download, inspect, then run

Piping to bash is a convenience for trusted environments. For a cautious path:

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/kanzifucius/xp-clean/main/scripts/install.sh
less install.sh
bash install.sh
```

### Options

| Mechanism | Purpose |
|-----------|---------|
| `XP_CLEAN_VERSION` / `--version` | Pin tag (`v0.1.0`, `v0.1.0-rc.1`, …); omit for latest non-prerelease |
| `XP_CLEAN_INSTALL_DIR` / `--install-dir` / `BINDIR` | Override install directory (default `~/.local/bin`) |
| `GITHUB_TOKEN` / `GH_TOKEN` | Optional — API rate limits, GHES, or a private fork |

The installer downloads the platform archive and `checksums.txt` from the same release, verifies SHA-256 strictly, and installs only the `xp-clean` binary.

Supported platforms match GoReleaser archives: Linux/Darwin × `x86_64` / `arm64`.

### PATH

If `~/.local/bin` is not on your `PATH`, the script prints a hint. Add it yourself (the installer does not rewrite shell rc files):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### CI tip

For reproducible pipelines, prefer a **pinned** tag or `gh release download` of a specific archive over an unpinned pipe-to-bash.

### Manual download

Browse [GitHub Releases](https://github.com/kanzifucius/xp-clean/releases), download the matching `xp-clean_<version>_<Os>_<Arch>.tar.gz` and `checksums.txt`, verify the checksum, extract, and place `xp-clean` on your `PATH`.

## Run in-cluster (private GHCR)

```bash
# PAT or fine-grained token with read:packages (SSO-authorise if the org requires it)
echo "$CR_PAT" | docker login ghcr.io -u USERNAME --password-stdin
docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0
```

In-cluster pulls need an `imagePullSecret` whose token can `read:packages` for `ghcr.io/kanzifucius/xp-clean`.

Use the **same SemVer tag** for the CLI archive and the image (for example `v0.1.0`) so operators can correlate laptop and cluster versions.

## Helm chart (in-cluster CronJob)

The chart at [`deploy/helm/xp-clean`](https://github.com/kanzifucius/xp-clean/tree/main/deploy/helm/xp-clean) installs into the dedicated **`xp-clean`** namespace:

- Namespace, ServiceAccount
- ClusterRole / ClusterRoleBinding (lab-oriented broad rules from the RBAC sketch; **no claim RBAC**)
- Role / RoleBinding for reading policy ConfigMaps in `xp-clean`
- Sample labelled policy ConfigMap (`xp-clean.io/policy=true`)
- **CronJob** running `xp-clean run --once` (default install path)
- Optional `manager` Deployment (`manager.enabled`, **off by default** — interval stub only)

There is **no** CleanupPolicy CRD in this chart. ConfigMaps remain the primary in-cluster policy surface.

### Install from a repository checkout

```bash
kubectl create namespace xp-clean
kubectl create secret docker-registry ghcr-xp-clean \
  --docker-server=ghcr.io \
  --docker-username=USERNAME \
  --docker-password="$CR_PAT" \
  -n xp-clean

helm upgrade --install xp-clean ./deploy/helm/xp-clean \
  --namespace xp-clean \
  --set image.tag=v0.1.0-rc.1 \
  --set imagePullSecrets[0].name=ghcr-xp-clean
```

Render without applying:

```bash
helm template xp-clean ./deploy/helm/xp-clean --namespace xp-clean
```

### Important values

| Value | Default | Purpose |
|-------|---------|---------|
| `image.tag` | `v0.1.0-rc.1` | Pin the GHCR image (prefer immutable `:vX.Y.Z`) |
| `imagePullSecrets` | `[]` | Required for private GHCR pulls |
| `dryRun` | `true` | Passed as `--dry-run` on the CronJob / manager |
| `cronjob.schedule` | `0 */6 * * *` | Kubernetes CronJob schedule |
| `cronjob.enabled` | `true` | Default ops path |
| `manager.enabled` | `false` | Optional secondary Deployment |
| `resources` | small requests/limits | Pod resource settings |
| `policy.create` | `true` | Install the sample policy ConfigMap |

Live deletes need `dryRun: false` in **both** chart values and the policy ConfigMap document. Edit selectors on the sample policy before enabling deletes. Full reference: the chart [`README.md`](https://github.com/kanzifucius/xp-clean/blob/main/deploy/helm/xp-clean/README.md) and [`values.yaml`](https://github.com/kanzifucius/xp-clean/blob/main/deploy/helm/xp-clean/values.yaml).

## Cut a release (maintainers)

1. Ensure `main` is green.
2. Tag and push (do not create the Release in the UI):

```bash
git checkout main && git pull
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

3. Watch the `release` workflow; confirm GitHub Release assets and `docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0`.
4. Spot-check `xp-clean version -o json` from the archive or image.

The docs workflow also deploys a **versioned** site for that tag (see [Home](index.md) for `latest` / `dev` aliases).

Release notes and assets: [GitHub Releases](https://github.com/kanzifucius/xp-clean/releases).

## Licence

SPDX / licence packaging for OCI labels is deferred for now — revisit when the repository adds an explicit `LICENSE`.
