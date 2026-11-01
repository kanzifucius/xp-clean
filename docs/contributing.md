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

Kind end-to-end (optional, local-first):

```bash
make e2e
```

Requires Docker, kind, kubectl, and helm. Details: [`test/e2e/README.md`](https://github.com/kanzifucius/xp-clean/blob/main/test/e2e/README.md). CI’s required check stays the job named **`test`**; kind e2e is not required on GitHub runners.

## Documentation site

```bash
python3 -m pip install -r requirements-docs.txt
mkdocs build --strict
mkdocs serve   # http://127.0.0.1:8000
```

Site layout (Material tabs): **Home → Install → Concepts → Architecture → CLI → Operations → Contributing**.

- Theme tokens and Archify iframe styles: `docs/stylesheets/extra.css`
- Architecture diagrams: standalone HTML under `docs/architecture/` plus caption pages that embed them
- Versioned deploys use [mike](https://github.com/jimporter/mike) on the `gh-pages` branch (see `.github/workflows/docs.yml`)

Prefer editing Markdown under `docs/` and `mkdocs.yml` on a feature branch; CI builds docs on PRs that touch the docs toolchain.

## Conventions

- British English in user-facing docs.
- Prefer ConfigMap / file policy samples that match locked product decisions (eligibility labels/globs, `instances-only` default, Teams notify schema, no claim support).
- Do not invent a second diagram system — regenerate Archify HTML when diagrams need updating.
