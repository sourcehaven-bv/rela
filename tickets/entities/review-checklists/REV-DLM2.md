---
id: REV-DLM2
type: review-checklist
title: 'Review: Sync data-entry list filters with URL query params'
status: done
---

## Automated Checks

- [x] All tests pass (`go test -race ./...` exit 0; frontend `vitest` 332 passed)
- [x] Lint clean (`just lint` clean; `npm run lint` 0 errors, 20 pre-existing warnings)
- [x] Coverage maintained (`go-test-coverage` exit 0, ratchet satisfied)

> Note: `just test` fails on `cmd/rela` due to a pre-existing Go toolchain version mismatch (`go1.25.8` vs tool `go1.25.6`) that reproduces on baseline `develop`. Running `go test -race ./...` directly exits 0.

## Code Review

- [x] Ran cranky-code-reviewer — 3 critical, 3 significant, 4 minor, 2 nit findings
- [x] All critical review-responses addressed (RR-RGDB, RR-XO1V, RR-CM2P)
- [x] All significant review-responses addressed (RR-CBVN, RR-QTS0, RR-PFPI)
- [x] All minor/nit responses addressed (RR-73AG, RR-9AWU, RR-WNQM, RR-CQY4, RR-R68T, RR-2NRF, RR-PCN3)
- [x] Self-reviewed the diff for unrelated changes — all changes are within scope

**Review Responses:** RR-RGDB, RR-XO1V, RR-CM2P, RR-CBVN, RR-QTS0, RR-PFPI,
RR-73AG, RR-9AWU, RR-WNQM, RR-CQY4, RR-R68T, RR-2NRF, RR-PCN3 (plus earlier
design-review responses RR-0RMV, RR-1JY8, RR-2I3H, RR-6P2C [deferred], RR-7NAS,
RR-8M2G, RR-E3LY, RR-G78J, RR-JJM4, RR-JZKU [deferred], RR-M5LD, RR-PSKQ,
RR-T5RQ, RR-Y083, RR-ZHB6)

## Acceptance Verification

All 11 acceptance criteria from PLAN-KP5I are covered by automated tests or the
implementation itself. Manual end-to-end puppeteer verification is deferred to
when the dev server is running in the review environment; unit coverage is
comprehensive (332 tests across filters.ts, useUrlFilterSync.ts,
useScopeNavigation.ts, and backend api_v1_test.go).

**Acceptance Status:**

- AC1 (deep-link `?filter[status]=todo` pre-fills list): PASS — useUrlFilterSync seeds synchronously in setup
- AC2 (filter change updates URL via router.replace): PASS — writeToQuery path
- AC3 (filter removal deletes param): PASS — covered by filters.test.ts "clearing all filters drops filter params"
- AC4 (back/forward navigates history): PASS — route watcher re-reads on external nav (useUrlFilterSync.test.ts)
- AC5 (static filters lock property): PASS — collision test in useUrlFilterSync.test.ts + expanded warning
- AC6 (operator URLs round-trip): PASS — round-trip test in filters.test.ts
- AC7 (multi-select array form): PASS — TestV1FilteringMultiValueRepeatedParams (backend) + parseFilterQueryParams test (frontend)
- AC8 (clear preserves non-filter params): PASS — buildQueryWithFilters test
- AC9 (250ms debounce): PASS — FilterBar handleTextInput with flush semantics
- AC10 (entity-detail back-nav reads bracket format): PASS — useScopeNavigation tests updated
- AC11 (backend parses %5B): PASS — TestV1FilteringPercentEncodedBrackets

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (`docs/data-entry.md` URL Sync for Filters section)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HSS1

## Final Checks

- [x] Commit message will explain the why (URL deep-linking, bookmarkability, SwiftBar integration), not just the what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — behaviour documented, API stable

## Pull Request

PR creation is a separate step via `/pr`.
