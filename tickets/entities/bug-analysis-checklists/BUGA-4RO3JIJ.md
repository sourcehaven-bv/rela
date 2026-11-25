---
id: BUGA-4RO3JIJ
type: bug-analysis-checklist
title: 'Analysis: Deep-linked list filter on a hyphenated relation name is dropped'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Steps: run `rela-server -project prototypes/data-entry/project` and open
`/list/all_tickets?filter[belongs-to]=frontend`. The `/api/v1/tickets` request
carries no `filter[belongs-to]`. `filter[status]=open` is sent. Picking the
value in the filter control works, because that path does not re-parse the URL.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: accept every name `metamodel.ValidateSchemaName` accepts and reject the
three prototype keys. Tests: vitest cases in `filters.test.ts` for legal names
(hyphen, dot, space, digit start, non-ASCII) and for rejected names. Related:
`conditions.ts` `IDENT_RE` is the expression tokenizer, not a name gate; no
other identifier allowlist parses URL keys.
