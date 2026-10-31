# Contributing to xp-clean

Thanks for helping improve xp-clean. By participating you agree to follow our
[Code of Conduct](CODE_OF_CONDUCT.md).

Product and docs conventions (British English, ConfigMap policy samples, no
LegacyCluster claims) are summarised in
[`docs/contributing.md`](docs/contributing.md).

## Development setup

Requirements: Go matching [`go.mod`](go.mod), `make`, and optionally
`golangci-lint` (CI uses the version pinned in `.github/workflows/ci.yml`).

```bash
git clone https://github.com/kanzifucius/xp-clean.git
cd xp-clean
make test
make build
```

Docs site (optional locally):

```bash
python3 -m pip install -r requirements-docs.txt
mkdocs serve
```

## Pull requests

1. Open a feature branch from the latest `main`.
2. Keep changes focused; one concern per PR when practical.
3. Ensure CI is green — the required status check is the job named **`test`**
   from the `ci` workflow.
4. Fill in the pull request template.
5. CODEOWNERS routes review to `@kanzifucius`.

### Solo maintainer note

GitHub does not count a PR author’s own approval toward a required review.
While the project is solo-maintained, branch protection should still **require a
pull request** and the **`test`** status check. Requiring one approving review
(and CODEOWNERS) is desirable once a second reviewer exists; until then the
maintainer may use an admin bypass on the ruleset, or keep required approvals at
zero while keeping PR + checks mandatory. See the OSS hygiene PR / maintainer
notes for the exact ruleset payload.

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## Licence

Contributions are licensed under the [Apache License 2.0](LICENSE).
