# Contributing

This is a personal family finance app. External contributions are welcome for bug fixes and improvements, but new features should be discussed first via an issue.

## Getting started

```bash
git clone https://github.com/TheIronRock95/home-finance.git
cd home-finance
cp .env.example .env
make dev
```

See `README.md` for full setup instructions.

## Workflow

1. Open an issue describing the bug or feature
2. Fork the repo and create a branch: `git checkout -b fix/description`
3. Make your changes (see conventions in `AGENTS.md`)
4. Run tests: `make test`
5. Run type check: `cd frontend && npx tsc --noEmit`
6. Commit using [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`
7. Open a PR — fill in the PR template

## Code conventions

See `AGENTS.md` for the full list. Key points:

- Money = integer cents everywhere, never floats
- All UI strings through `react-i18next` (both `en.json` + `nl.json`)
- No `Co-Authored-By` or AI attribution in commits
- No comments unless the WHY is non-obvious

## Database migrations

Add a new goose SQL file in `backend/migrations/`. Never edit existing migrations.

```
backend/migrations/0004_my_change.sql
```

## Releases

Releases are fully automated via release-please. Merge to `main` → release-please opens a Release PR → merge that → tag + image + CHANGELOG generated automatically.
