---
id: DOCS-NMJ76NU
type: docs-checklist
title: 'Docs: Two-way sync: send deletes with guards'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`deletes.go` has a package-level note on the deleting state and on why a 404
answer is not counted. `guarder`, `goner` and `deleter` each document their
phase. `Resolve` documents the two delete reasons and when a resolve of a
remote-deleted relation is refused.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README surface changed)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new code pattern)
- [x] Help text accurate (if CLI changes)

`rela remote pull --allow-gone` help names both directions. The report prints
the deleted counts and the deletes waiting to be sent.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the project has no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable)

`docs/data-entry/api-reference.md` has a DELETE section on If-Match. The remote
sync guides cover `send_deletes`, the shared `max_gone`, the delete conflict
reasons and who may resolve them. The ACL and CLI guides are updated. `just
docs` regenerated `docs/`.
