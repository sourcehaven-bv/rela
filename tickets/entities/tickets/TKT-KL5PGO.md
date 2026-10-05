---
id: TKT-KL5PGO
type: ticket
title: 'conflict resolve: run the affordance gates and keep the tail face'
kind: chore
status: backlog
---

## Description

Conflict resolution (the screen that resolves a file with merge conflicts, file
backend only) applies the ACL but not the affordance gates. A relation file
skips `relationSources` / `relationOpDenial` / `relationMetaDenial`, and an
entity file skips the field affordance gate. Every other write path in
`internal/dataentry` runs them, so `_actions` and `when:` rules can be bypassed
here.

The conflict parser (`internal/conflict/parse.go`, `docToRelation`) drops the
edge's tail face. The ACL therefore judges a faced source on every declared
face. That is stricter than a normal save, never weaker, but it refuses writes
the save allows.

## Acceptance

- The parser keeps the tail face.
- A resolved relation file runs the same relation affordance gate as
`PATCH /relations/...`; a resolved entity file runs the field gate.
- Denials do not name hidden faces.
- Tests through the resolve route, in the style of
`relation_sibling_paths_test.go`.
