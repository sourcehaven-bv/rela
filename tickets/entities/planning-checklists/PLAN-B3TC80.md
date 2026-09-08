---
id: PLAN-B3TC80
type: planning-checklist
title: 'Planning: `_self` on a non-bare face 404s under a configured default_world'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `ID@face` accepted on GET/PATCH/DELETE/`_views`; face delete;
`_faces[].ref`; view-row `_self`; SPA writes to `_self`, affordances from
`_actions`, form loads its address, face switcher by address, copy
toast/landing, world note only for faced types. Out: operator-configurable world
chrome text and redirect targets (atlas issue 6), relation writes for
content-scoped edges on non-bare faces (refused with 422),
attachments/sub-resources on faced ids.

**Acceptance Criteria:**
1. `GET /policys/POL-1@published` returns 200 with matching `_self` under a configured `default_world` (atlas verify manual, last section).
2. The SPA offers Edit/Delete/copy wherever `_actions` says so, under any world; a stand-in face with `update: false` shows a note naming the bare face.
3. Edit from a concept page opens an editable form for the concept.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the design is given by the atlas findings and the existing address grammar)
- [x] ~~Searched for existing libraries~~ (N/A: no library problem)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `entity.ParseStateRef` / `FormatStateRef` already define
the address grammar; `selfHref` already emits it; BUG-Y0GNSB already authorizes
faced writes. `store.DeleteEntityState` existed with no manager caller.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Parse the address once at the HTTP boundary
(`entityRef`), gate rows on the bare id and faces on the face, serve explicit
addresses literally under any world. SPA: one util `entityRef(e)` reads `_self`;
every write uses it; every gate is `_actions`. Alternative rejected: emitting
`_self` as `?world=` URLs (a world is a read-side rule, not an address).

**Files to modify:**
internal/dataentry/{entityref,visiblereader,entityreader,api_v1,write_handler,views,viewworld,views_handler,sections,affordances}.go,
internal/entitymanager/manager.go, internal/apiwire/v1/responses.go; frontend:
utils/entityRef.ts, EntityDetail.vue, EntityList.vue, KanbanView.vue,
CalendarView.vue, DynamicForm.vue, HistoryView.vue, useCreateTarget.ts,
useListActions.ts, stores/entities.ts.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Path segment `ID@face`: `entity.ParseStateRef`
grammar; anything rejected is the uniform not-found. Declared face names map
through `metamodel.StoredFace`.

**Security-Sensitive Operations:** Row gate on the bare id before any store
read; face gate (`faceReadable`) after; a denied world still blocks explicit
addresses; face delete authorized with the face in the subject
(GrantsVerbOnState); content-scoped relations refused on a face address so an
edge cannot be misfiled onto the bare tail.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `entityref_test.go` (GET/PATCH/DELETE/_views by address,
face gate, 404 uniformity); `manager_deleteface_test.go`; SPA world test suites
rewritten; atlas verify manual end to end; UI checklist walked in Chrome.

**Edge Cases:** `ID@<bare_face>` alias; undeclared face name; `POL-1@@`,
uppercase; bare face without declared name (`ref` falls back to bare id, SPA
names the default world).

**Negative Tests:** Bare delete grant on `ID@published` → 403 and
`_actions.delete: false`; `policy@published` reader on `ID@draft` → 404.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Removing the SPA lock relies on `_actions` being right per face —
pinned by the existing affordance contract test plus the new faced tests.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes
- [x] docs/content-states.md, docs/metamodel.md

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design was reviewed through the atlas findings document and its executable manual)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** N/A
