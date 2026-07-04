## What

<!-- One-line description of the change -->

## Why

<!-- What problem does this solve? Link the issue if applicable: Fixes #123 -->

## How

<!-- Brief explanation of the approach. Skip if obvious from the diff. -->

## Checklist

- [ ] `make test` passes
- [ ] `cd frontend && npx tsc --noEmit` passes
- [ ] New UI strings added to both `en.json` and `nl.json`
- [ ] New DB changes are in a new goose migration file (never edited existing)
- [ ] Money amounts are integer cents (no floats)
- [ ] Audit log called for any destructive/state-changing action
