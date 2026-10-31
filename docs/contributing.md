# Contributing

Source, issues, and pull requests live in the GitHub repository:

**[https://github.com/kanzifucius/xp-clean](https://github.com/kanzifucius/xp-clean)**

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
