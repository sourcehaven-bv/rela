---
id: DOCS-RLVY25I
type: docs-checklist
title: 'Docs: Audit read API and Changes tab'
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the lifecycle fence and exact-face match in `sqlaudit.Log.ForEntity`, `LifecycleStart`, the face fold in `subjectRows`, the start check and face filter in `visibility.EntityChanges`
- [x] Function/type docs if public API: godoc on `visibility.AuditEntries`, `ChangesQuery`, `EntityChanges`, `audit.StartsLifecycle`, `audit.CopyStateSummary`, `dataentry.SetAuditLog`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the history-read and visibility-wrapper rules as written)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-audit-log.md`: the `audit-read` op, face columns, "In the web app", jq examples; regenerated
- [x] `GUIDE-acl-security.md`: `audit:read`, the Changes tab gating and its accepted limits; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `docs/data-entry/api-reference.md` section on `_changes` and `_audit` (the OpenAPI spec does not describe the history routes either)
