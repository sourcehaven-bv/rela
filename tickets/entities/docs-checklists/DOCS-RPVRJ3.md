---
id: DOCS-RPVRJ3
type: docs-checklist
title: 'Docs: Cut CI wall clock: de-serialize the build tail and fix Go cache thrash'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments explain why, not what

The two non-obvious decisions are documented where the next editor of the file
will meet them, rather than only in this ticket:

- `.github/workflows/ci.yml` — why the module cache stays shared on setup-go
while the build cache gets a per-job key, including the observed 10MB-791MB
spread that proved the old single key was serving useless caches.
- `.github/workflows/ci.yml` — why the `rela-tickets` cache step carries the
same `if:` as its checkout (empty `hashFiles` collapses the key onto its own
restore-key prefix).
- `.github/workflows/ci.yml` — why there is deliberately no `build` job, and
the constraint on reintroducing one.
- `e2e/playwright.config.ts` — what is and is not isolated per worker, and
that `retries: 2` will mask load flake as a slow green.

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-facing change)
- [x] ~~Architecture docs updated~~ (N/A: no architecture change; CI job
topology is not described in `docs/`)
- [x] ~~CLAUDE.md updated~~ (N/A: no new rule for contributors. The Commands
section already points at the `justfile`, and nothing about how to run the
checks locally changed)

## External Documentation

- [x] ~~User guide updated~~ (N/A: CI-internal)
- [x] ~~API docs updated~~ (N/A: no API change)
- [x] ~~Changelog entry~~ (N/A: no shipped behaviour change; releases come
from GoReleaser and this alters neither the binaries nor their contents)
