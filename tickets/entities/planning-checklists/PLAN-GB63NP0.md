---
id: PLAN-GB63NP0
type: planning-checklist
title: 'Planning: Relation filter controls match on target id instead of display title'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In:
- Server: a relation filter value matches a readable neighbour whose **id**
equals it (`matchRelationFilterMany`, `internal/dataentry/api_v1.go`).
- Server: a value that matches no readable neighbour id still matches on the
redacted display title, as today. This keeps old URLs working. Deprecated in the
docs.
- SPA: `EntityTargetSelect` commits the candidate id. One option per entity
(no collapse by title). Every label is `Title (ID)`
(`entityDisplayTitleWithId`), so typeahead search by id keeps working. The
typeahead's committed label is the matching option's label, or the raw value
when no candidate matches.
- Server: a debug log when a row matched only through the title fallback, with
the relation and entity type but not the value. It shows when title matching can
be removed.
- Hosts: lists, kanban, calendar and gantt all use `FilterBar` and the one
server path, `applyRelationFilters`. Calendar takes property controls only.
- Docs: `GUIDE-data-entry.md` relation filter section; regenerate `docs/`.

Out:
- Store pushdown of relation filters (FEAT-R012DX). Id matching is the
precondition; the pushdown is TKT-2N9T4TQ / TKT-QA4LLQC territory.
- Removing title matching. A later ticket once old URLs are gone.
- New operators. Still `eq` / `ne`.

**Acceptance Criteria:**
1. Two targets titled "Done": `filter[has_status]=<id A>` returns only rows
linked to A. Test: Go list-API test with two same-titled statuses.
2. Renaming a target keeps `filter[rel]=<id>` results unchanged. Test: rename
the target in the test store, re-query.
3. `filter[rel]=<title>` still matches (legacy URL). Test: existing title tests
keep passing.
4. An id of an unreadable neighbour never matches, for `eq` (no row) and `ne`
(all rows). Test: ACL fixture with a hidden target.
5. The SPA select emits the id and shows titles. Test: Vitest on
`EntityTargetSelect` and `FilterBar`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small change; RES-8CKUNJ and RES-7SH71TW cover it)
- [x] ~~Searched for existing libraries~~ (N/A: internal matching rule)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal matching rule)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-8CKUNJ, RES-7SH71TW

**Existing Solutions:**
- `matchRelationFilterMany` already reads only served (row-gated) neighbours
and compares the redacted title; the id compare slots in beside it.
- `EntityTargetSelect` is used only by `FilterBar`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** In `matchRelationFilterMany`, an edge matches when its
served neighbour has `h.ID == want`, or (legacy) its redacted display title
equals `want`. The served-set and face checks stay as they are, so unreadable
neighbours never match. In the SPA, option value = `c.id`, label =
`entityDisplayTitleWithId(c)`; the typeahead's committed label is looked up from
the options. A deep-linked value that is no candidate's id is shown raw, as
today.

Alternatives rejected:
- A separate key (`filter[rel][id]=`). It would need a second URL grammar
and parser change; ids and titles rarely collide, so one key is enough.
- Upgrading a legacy title in the SPA to an id. When two targets share the
title the old URL matched both; picking one would silently change results.

**Files to modify:**
- `internal/dataentry/api_v1.go`, `internal/dataentry/list_pipeline.go` (comments)
- relation filter tests in `internal/dataentry`
- `frontend/src/components/common/EntityTargetSelect.vue` (+ test)
- `frontend/src/components/lists/FilterBar.vue` (comment) (+ test)
- `docs-project/entities/guides/GUIDE-data-entry.md`, regenerated `docs/`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `filter[<relation>]` query value from the URL. Only relation keys a
configured relation FilterControl declares reach this pass
(`relationFilterClassifier`). The value is compared, never interpolated.

**Security-Sensitive Operations:**
- Existence oracle: an id compare could confirm that a row links to an entity
the reader cannot see. Prevented because only neighbours in the served
(row-gated) set are compared, as for titles today. `ne` with a hidden id returns
every row, the same as for an id that does not exist.
- Field redaction: an id is not a redactable field, so comparing it discloses
nothing `visible:` hides. The title fallback keeps comparing the redacted title.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria 1-5; Go tests run through the list
HTTP handler with the real visibility gate.

**Edge Cases:**
- Empty value: skipped, as today.
- Value equal to one target's id and another's title: both match (documented).
- Incoming direction: id compare respects `ownedByFace`.
- Faced types: the id is the bare id; any served face of it matches.

**Negative Tests:**
- Hidden target id with `eq` and `ne` (criterion 4).
- Unknown operator still 400.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Saved links with titles: kept working by the fallback.
- Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via `GUIDE-data-entry.md`)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-3LELYWD (significant), RR-HCY6FY1, RR-H0Y7P2Q,
RR-QFRJSD6 (minor), RR-RF37Y0D (nit). All addressed in this plan.
