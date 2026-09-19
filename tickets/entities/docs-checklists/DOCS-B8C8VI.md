---
id: DOCS-B8C8VI
type: docs-checklist
title: Documentation
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — the corrected `store.EntityWriter`
comment now explains WHY a default-face delete is allowed (BUG-HC6I2T removed
the invariant; the refusal would make flat→faced impossible) and names the test
that pins it
- [x] ~~Function/type docs if public API~~ (N/A: no API change in this ticket)

## Project Documentation

- [x] README updated (if applicable) — regenerated via `just docs`
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI change in this ticket; the command it
references ships with TKT-FTOENU)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no changelog file)
- [x] API docs updated (if applicable) — `GUIDE-data-migration.md` corrected at
the source and `docs/data-migration.md` regenerated from it
