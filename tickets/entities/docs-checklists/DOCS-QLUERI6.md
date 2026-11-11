---
id: DOCS-QLUERI6
type: docs-checklist
title: 'Docs: TKT-8ZGAEC5'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the merge-then-push order in `relsync.go`, why a failed merge stops the push, why `_updated` on a relation needs `RelationMetaRestricted`, the clock bounds in `rules.go`
- [x] Function/type docs if public API: godoc on `RelationMetaRestricted`, `RelationMetaGatedBy`, the `Pusher` errors and the conflict rule constants; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela remote resolve --dismiss` help and argument errors

## User-facing Documentation

- [x] `docs/remote-sync.md`: conflict rules, `max_clock_skew`, `latest` fallbacks, superseded entries, `--dismiss`, relation push, purge statement
- [x] Security guide: `GUIDE-acl-security.md` covers `_updated` (whole row only; relation restriction), the relation PATCH `If-Match`, clock trust and gated relation types; regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `docs/data-entry/api-reference.md` (single relation GET/PATCH, `_updated`) and OpenAPI (relation GET, PATCH with If-Match, 400, 412)
