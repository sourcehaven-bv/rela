---
id: IMPL-MO3SFSC
type: implementation-checklist
title: 'Implementation: Content-free keyset pages for search with no text'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (storetest RunHeaderPageTests, faceselect header checks, recorder forwarding and BodyWatch fallback tests, archguard pageable cases)
- [x] Integration tests written (test full flow, not just units) (search listall_bodies_test: no-text search and SearchVisible over BodyWatch, plus header-vs-entity page differential through search.Service)
- [x] Happy path implemented
- [x] Edge cases from planning handled (worlds with fallback and exclude, face ceiling, AllFaces resuming mid-family, empty store, invalid cursor, mutation isolation, decorators over a store without the capability)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (storetest Factory, bodyStore seed helper)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings (header pages compared against store.HeaderOf of the entity page)

## Manual Verification

- [x] Feature manually tested end-to-end (budget tests run with typePages reverted to ListEntitiesPage: both fail; restored: both pass)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** go test -race passes for memstore, fsstore, store,
storetest, search, archguard, visibility (default build); pgstore and search
under -tags postgres against a scratch postgresql@17, including EXPLAIN cases
showing the header page uses entities_type_id_face_idx and selects no content;
sqlitestore under -tags sqlite including EXPLAIN cases.

## Quality

- [x] Code follows project patterns (check similar code) (optional capability plus dispatcher with fallback, like HeaderReader and GraphHeaderQueryer; narrow EntityPager interface at the call site)
- [x] Checked for DRY opportunities (pgstore and sqlitestore share one generic queryEntityPage per backend for entity and header pages; memstore shares pageOf)
- [x] No security issues introduced (store-level read; gating stays in the visibility wrappers, and search still applies its scope)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
