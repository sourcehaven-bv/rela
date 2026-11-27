---
id: BUG-NZN2SP
type: bug
title: 'Conflict resolution ignores the fields: write policy'
description: 'authorizeConflictResolve checks only the update ACL. A conflict resolution writes the whole entity file, so it can set a field that fields: makes read-only. Found in the review of BUG-1N7OU9.'
priority: medium
effort: s
status: backlog
---

## Description

`authorizeConflictResolve` (`internal/dataentry/write_handler.go`) re-authorizes
a conflict resolution with the `update` ACL only. Conflict resolution bypasses
entitymanager and writes the whole entity file, so it never reaches
`validateFieldWrite`. A principal with `update` can resolve a conflict with a
version that changes a field the `fields:` policy makes read-only.

## Fix

Diff the resolved entity against the stored one and run `validateFieldWrite` on
the changed and removed properties before the write, as PATCH does.

## Merged from TKT-KL5PGO (2026-11-27)

Conflict resolution also skips the affordance gates. A relation file skips `relationSources`, `relationOpDenial` and `relationMetaDenial`; an entity file skips the field affordance gate. The conflict parser (`internal/conflict/parse.go`, `docToRelation`) drops the edge's tail face, so the ACL judges a faced source on every declared face.

Also required:

- The parser keeps the tail face.
- A resolved relation file runs the same relation affordance gate as `PATCH /relations/...`; a resolved entity file runs the field gate.
- Denials do not name hidden faces.
- Tests through the resolve route, in the style of `relation_sibling_paths_test.go`.

## Note (2026-11-27)

Mostly fixed in c6fba457e (#2100) and 80ba3d766 (#2097). `conflictFieldDenial` runs the field check on entity files and `relationSources` plus `relationMetaDenial` on relation files; the handler restores the tail face from the file path. `relationOpDenial` does not apply, because a resolve updates an edge. Remaining: keep the tail face in `docToRelation` (`internal/conflict/parse.go`), and add a resolve-route test with a faced source that checks denials do not name hidden faces.
