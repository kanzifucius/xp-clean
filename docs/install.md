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

GHCR packages stay **private** (match the private repository).

## Pull a private image

```bash
# PAT or fine-grained token with read:packages (SSO-authorise if the org requires it)
echo "$CR_PAT" | docker login ghcr.io -u USERNAME --password-stdin
docker pull ghcr.io/kanzifucius/xp-clean:v0.1.0
```

In-cluster pulls need an `imagePullSecret` whose token can `read:packages` for `ghcr.io/kanzifucius/xp-clean`.

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
