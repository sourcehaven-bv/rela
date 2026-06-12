---
id: IMPL-Y3CEK5
type: implementation-checklist
title: 'Implementation: ACL read-side: /_search visibility — VisibleSearcher seam, generic + pgstore-native impls, conformance suite'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Commits 9a3f04eb (seam + generic + conformance + dataentry gate) and 5bea3431
(pgstore native), branch feat/acl-search-tkt-ba8bsx stacked on PR 949.

- **Conformance (AC5/AC6)**: `storetest.RunVisibleSearchTests` — 12 cases (the planned 8 plus EmptyScope, WildcardQueryInvalid, EmptyText, and per-case no-leak + ordered-subsequence invariants) — green on FOUR combinations: generic+memstore/linear, generic+fsstore/linear, generic+memstore/bleve (`TestVisibleConformance_Bleve`), pgstore-native against a real database (`RELA_TEST_DATABASE_URL=...rela_test go test -tags postgres ./internal/store/pgstore/ -run TestConformance/VisibleSearch` → PASS 12/12, first run).
- **Dataentry HTTP tests (AC1–4, 3b, 7, 7b, 9)**: `acl_search_test.go` — 8 tests, all green: type-level grant + denied-principal empty shape + raw-body leak assertions; editor-of/belongs-to inheritance; visible-hit-related-to-hidden (serializer leak, includeRelations=false pin); DenyAll short-circuit (0 backend calls) + positive control (exactly 1); scope-error mapping (500 acl_query_failed, constant detail, synthetic `pq: relation "secret_internal_acl_table"` asserted absent, errors.Is(errACLListQuery)); canceled-ctx silent on `_position?scope=search`; backend-error mapping (500 search_failed, path string not echoed, NOT errACLListQuery); NopACL regression incl. off-metamodel `ghost` type staying visible (wildcard scope).
- **AC8**: pre-existing `TestACLPosition_SearchScopeGated` green unchanged after `readableSubset` retirement; full dataentry suite green (no regressions).
- **Live end-to-end (real server, real acl.yaml, .ignored/vmd8-verify, --principal-header X-Rela-User)**: alice (editor-of PRJ-42) `GET /_search?q=Hidden` → `[] total 0` (all 5 matching tickets belong to PRJ-9); `q=Visible` → exactly [TKT-V01..V05] total 5; mallory (no roles) `q=Visible` → `[] total 0`; mallory raw body for `q=Hidden` contains zero `TKT-H` occurrences.
- **Builds**: default, `-tags memorybackend`, `-tags postgres` all compile; commit 1 verified buildable standalone (postgres recipe derives generic until commit 2 flips native).
- **`just ci`**: lint + arch-lint + tests + coverage green (single failure: `e2e/node_modules/flatted/...` 0% — a node_modules artifact in the local checkout, gitignored, absent in CI).
- **Docs (AC10)**: GUIDE-acl-security gained "Global search (/_search, TKT-BA8BSX)" section (scope lookup, post-visibility limit, generic-vs-native, bleve 10k caveat, deny short-circuit, serializer invariant, error semantics); `_search` removed from "What still leaks"; `docs/acl-security.md` regenerated via `just docs`; CLAUDE.md tests section points new VisibleSearcher impls at the conformance suite.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `ResolveTypeScope` shared by both impls; `buildPredicateSQL` reused (not duplicated) for the native SQL; error-sentinel taxonomy reused from VMD8 rather than new codes
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned — and two PRE-EXISTING silent failures fixed: swallowed `runFreeTextSearchE` error and dropped iterator error on executeQuery's non-free-text branch)
- [x] No debug code left behind
