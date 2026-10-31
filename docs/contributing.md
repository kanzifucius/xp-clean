# Contributing

Source, issues, and pull requests live in the GitHub repository:

**[https://github.com/kanzifucius/xp-clean](https://github.com/kanzifucius/xp-clean)**

Repository-level contribution guide, Code of Conduct, and security reporting:
[`CONTRIBUTING.md`](https://github.com/kanzifucius/xp-clean/blob/main/CONTRIBUTING.md),
[`CODE_OF_CONDUCT.md`](https://github.com/kanzifucius/xp-clean/blob/main/CODE_OF_CONDUCT.md),
[`SECURITY.md`](https://github.com/kanzifucius/xp-clean/blob/main/SECURITY.md).

## Local checks

```bash
make test
make build
golangci-lint run ./...   # matches CI (.golangci.yml)
```

## Documentation site

```bash
python3 -m pip install -r requirements-docs.txt
mkdocs build --strict
mkdocs serve   # http://127.0.0.1:8000
```

Versioned deploys use [mike](https://github.com/jimporter/mike) on the `gh-pages` branch (see `.github/workflows/docs.yml`). Prefer editing Markdown under `docs/` and `mkdocs.yml` on a feature branch; CI builds docs on PRs that touch the docs toolchain.

## Conventions

- British English in user-facing docs.
- Prefer ConfigMap / file policy samples that match locked product decisions (eligibility labels/globs, `instances-only` default, Teams notify schema, no claim support).
