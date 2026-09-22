---
id: DOCS-Z1O775
type: docs-checklist
title: Documentation
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — `adoptface.go` states why the mapping
need not be exhaustive here (nothing is dropped, so an omission is recoverable)
and why this is not a migration file (no shape edge to hang one on)
- [x] Function/type docs if public API — `Adopt`, `AdoptDeps`, `AdoptRequest`,
`AdoptResult` documented; `newCapturer` extracted as a free function with the
reason stated

## Project Documentation

- [x] README updated (if applicable) — regenerated via `just docs`
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; the command is the fifth
raw-store exception under the terms CLAUDE.md already documents)
- [x] Help text accurate (if CLI changes) — kong help on `migrate adopt-face`
and its four flags

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no changelog file)
- [x] API docs updated (if applicable) — `docs-project/entities/guides/GUIDE-data-migration.md`
gains a "Repairing rows that are already stranded" section;
`docs/data-migration.md` regenerated from it

Also corrected in the same change (TKT-RJV4C0): the stale passage telling
operators to export-and-reimport a populated type, the `bare-row-on-faced-type`
remedy text in `internal/analysis/states.go`, and the `store.EntityWriter`
comment claiming a default-face delete is refused.
