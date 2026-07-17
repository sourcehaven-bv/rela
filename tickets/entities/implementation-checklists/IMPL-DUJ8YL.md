---
id: IMPL-DUJ8YL
type: implementation-checklist
title: 'Implementation: ACL read-side: close the /_search match-on-hidden-field oracle (drop hits matching only visible:-hidden fields)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written (MatchTextFields, bleve MatchedFields, MatchHasVisibleField)
- [x] Integration tests (conformance suite ×3 backends + e2e handler test)
- [x] Happy path implemented (backend provenance → seam intersection → drop)
- [x] Edge cases handled (id/content never-gated, empty scope, stale entity, missing provenance)
- [x] Error handling in place (ErrScope fail-closed on hidden-func error and missing provenance)

## Test Quality

- [x] Fixture builders used (entity.New + seedVisibleFieldWorld)
- [x] No hardcoded values where object in scope
- [x] Only test-relevant values specified
- [x] Interpolated values from objects
- [x] Property comparisons use search vocabulary constants, not raw strings

## Manual Verification

- [x] Feature verified end-to-end via `handleV1Search` test with policy resolver
- [x] Each acceptance criterion verified (see review checklist)
- [x] Edge cases verified (fail-closed, false-drop guard)

**Verification Evidence:** `go test -race` green across search / store /
storetest / dataentry / mcp / appbuild. Conformance `RunVisibleFieldSearchTests`
green on bleve + linear (pgstore DB-gated, runs in CI). E2e
`TestACLSearch_HiddenFieldOracleClosed` + control pass. Fail-closed pinned by
`TestSearchVisibleFields_FailsClosedWithoutProvenance`.

## Quality

- [x] Follows project patterns (VisibleSearcher seam, consumer-side interfaces, single-snapshot rule)
- [x] DRY: shared `MatchHasVisibleField`, `MatchTextFields` ground truth reused across backends
- [x] No security issues introduced (fail-closed everywhere; conformance-pinned)
- [x] No silent failures (ErrScope surfaced + returned)
- [x] No debug code left behind
