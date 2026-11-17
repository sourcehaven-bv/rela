---
id: DOCS-9HVPY4A
type: docs-checklist
title: 'Docs: TKT-ERO2X5G'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the revision is compared after the access checks, why undeclared stored fields are dropped, why a hidden owner gets no denied-write line, why filerecords writes the new month file first
- [x] Function/type docs if public API: godoc on `Mutator`, `Change`, `Service.Update`, `Service.Delete`, `ErrStale` and the new `Record` fields; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing optional-capability pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-records.md`: the `mutable` key, "Changing rows", reserved fields, PATCH and DELETE, 409 `stale_revision`, audit lines; regenerated with `just docs`
- [x] `GUIDE-acl-security.md`: rights, 404 and audit for changing a row

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the records API is documented in GUIDE-records only; there is no OpenAPI entry for it)
