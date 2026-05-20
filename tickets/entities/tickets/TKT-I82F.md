---
id: TKT-I82F
type: ticket
title: Investigate docs/data-entry/api-reference.md generation status
kind: docs
priority: low
effort: xs
status: backlog
---

## Goal

Clarify whether `docs/data-entry/api-reference.md` should be auto-generated from
a `docs-project/` entity, or remain a hand-written file. Either way, make the
situation unambiguous so future contributors know where to edit.

## Context

During TKT-Y72A phase 1 (PR #779), a section on `_actions` was added to
`docs/data-entry/api-reference.md`. The reviewer flagged this as
potentially-generated; investigation showed the file is currently hand-written
(`scripts/generate-docs.lua` only emits `guide`, `tutorial`, `scenario`
entities), and `just docs` didn't overwrite the manual edit.

The reviewer asked for a quick follow-up PR to "correct that" — meaning either:

1. **Convert api-reference.md to a generated doc**, sourced from a new
`guide`-type entity in `docs-project/entities/guides/`. Aligns the data-entry
docs with the rest of the docs pipeline; manual edits would move to the entity.
2. **Document the hand-written status** explicitly, e.g. a header comment
in the file or a note in `CONTRIBUTING.md` / docs-project README, so future
contributors don't waste time looking for a source.

Pick during the design phase. (1) is the more uniform answer; (2) is cheaper.

## References

- PR #779 (TKT-Y72A phase 1)
- `scripts/generate-docs.lua`
- `scripts/generate-docs.sh`
- `docs/data-entry/api-reference.md`
