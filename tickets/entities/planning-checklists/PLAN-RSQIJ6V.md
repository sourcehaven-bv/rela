---
id: PLAN-RSQIJ6V
type: planning-checklist
title: 'Planning: Content-free keyset pages for search with no text'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a keyset-paged header read in `internal/store`, implemented on
memstore, fsstore, pgstore and sqlitestore; forwarding in the storetest
decorators (Counting, BodyWatch, Breadth); conformance coverage;
`search.Service.typePages` switched to it; a body-read budget test; the
archguard unpaged-read matcher taught the new read. Out: text search
(`loadFaces` is already content-free; the gated field check needs the body for
match provenance), any change to paging order or cursors.

**Acceptance Criteria:**
1. A search with no text reads no body, on every query shape (one type, several,
every type, filters, a world, the admitting path, the gated search):
`TestSearch_NoTextReadsNoBodies`, `TestSearchVisible_NoTextReadsNoBodies`. Both
fail when `typePages` is reverted to `ListEntitiesPage`.
2. Results are unchanged: `TestSearch_NoTextHeaderPagesMatchEntityPages`
compares every query against a search over a reader with only
`ListEntitiesPage`; the existing `TestSearch_NoTextIsScaleFree` and
`TestSearch_NoTextAdmitsWholeFamilies` still pass.
3. A header page equals the entity page projected, rows and cursor, natively and
through the fallback, on every backend: `storetest.RunHeaderPageTests`.
4. pgstore and sqlite select no content column and keep their index plans:
header-page cases in both `TestFaceReadExplain` suites.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s, the ticket names the two options)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal store seam, follows in-repo patterns)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `store.HeaderReader` + `store.ListEntityHeaders` (store.go): optional capability
with a generic fallback dispatcher. The new read follows it.
- `store.GraphHeaderQueryer` + `store.GraphQueryHeaders`: same pattern.
- `search.Service.loadFaces` already reads headers for text search.
- Backends already share the page SQL builders between entity and header
column lists (pgstore and sqlite `buildEntitySelectSQL`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** New optional capability `store.HeaderPager` with
`ListEntityHeadersPage(ctx, EntityQuery) (Page[EntityHeader], error)` and a
dispatcher `store.ListEntityHeadersPage(ctx, EntityPager, q)` that falls back to
projecting `ListEntitiesPage`. The contract: same rows, order and NextCursor as
`ListEntitiesPage`, so cursors are interchangeable. Each backend shares one
generic free function between its entity page and header page (memstore and
fsstore `pageOf`, pgstore and sqlite `queryEntityPage` over the existing
`entityPageSQL`, now parameterised by column list). `typePages` calls the
dispatcher and builds the same content-free `*entity.Entity` that `loadFaces`
builds.

Alternatives rejected:
- A second method on `store.HeaderReader`: the visibility readers and other
HeaderReader implementers would stop satisfying the interface, and
`ListEntityHeaders` would silently take its fallback for them.
- An after-key on `GraphQuery`/`GraphQueryHeaders`: a new field every graph
backend, graphquerynaive and the visibility pushdown must honour, and its
interaction with `OrderBy`/`Offset` would need its own contract.

**Files to modify:** internal/store/store.go; memstore/memstore.go;
fsstore/entity.go, fsstore.go; pgstore/entity.go, pgstore.go;
sqlitestore/page.go, sqlitestore.go; storetest/{headerpage.go, faceselect.go,
storetest.go, counting.go, bodywatch.go, breadth.go}; search/index.go; tests in
search, pgstore, sqlitestore, archguard.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The query and cursor come from the same callers
as `ListEntitiesPage` and pass the same `storeutil.ValidateEntityQuery` and
`DecodeCursor` checks. A zero face selection is refused (conformance-pinned in
`RunFaceSelectionTests`).

**Security-Sensitive Operations:** None new. The search reader is the raw store,
as before; ACL gating of a search with no text stays in `q.Admit` and
`visibility.SearchScoped`, unchanged. A header carries the same
Redacted/Inaccessible fields as the entity it projects.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above.

**Edge Cases:** Pages ending inside a family (AllFaces), world fallback and
exclusion, a face ceiling, growing page sizes, cursors passed between the entity
and header reads, an empty store, limit 0.

**Negative Tests:** Invalid cursor fails; zero and unset face selections fail
with ErrInvalidQuery, natively and through the fallback; mutating a header's
Properties does not write through.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Order or cursor drift between the two reads: both share one function per
backend, and the conformance walk compares every step.
- A decorator hiding the capability: Counting, BodyWatch and Breadth forward it;
BodyWatch routes the fallback through its counted ListEntitiesPage.
- plimsoll caps: +1 exported method on each backend and on Counting, documented
at the directive.

Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal store and search change, no user-visible behaviour)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no docs impact)

**Documentation Impact:** N/A. Internal change; godoc on the new interface and
dispatcher.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort-s change whose two candidate designs the ticket names; the choice is reviewed in the code review instead)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
