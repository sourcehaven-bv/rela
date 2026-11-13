---
id: DOCS-J7Q9C4K
type: docs-checklist
title: 'Docs: TKT-J7Q9C4K'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: owner gate per face, ledger balance per commodity, id ordering within a batch, cursor caveat
- [x] Function/type docs if public API: package docs for records and its backends; comment-lint `doclink` gate clean

## Project Documentation

- [x] README updated (if applicable): guide row added by `just docs`
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: the side-store pattern follows piles and lease)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change; CLI is in TKT-EV8U8T4)

## User-facing Documentation

- [x] GUIDE-records: schema, ledgers, ACL, HTTP API, retention, storage, accepted limits; regenerated `docs/records.md` with `just docs`
- [x] Security guide: ACL and the accepted group-key channel are covered in GUIDE-records

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the HTTP API is documented in GUIDE-records; OpenAPI coverage follows with the SPA in TKT-EV8U8T4)
