---
id: IMPL-S85VKS
type: implementation-checklist
title: 'Implementation: Extract dataentry query/search leaf off App (92 → ~87), de-risking the read-pipeline steps'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; existing handler/ACL-search suites drive every moved helper)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled — see the closure/value decision below
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: call-site re-points, plus one deleted test whose subject was deleted)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (dataentry package + full suite, `-count=1`)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1470), re-checked by the coordinator:**
- App 92 → **86**; directive lowered to 86 with a history line. Count
verified excluding the one `*App` method that lives in watcher_test.go.
- `just plimsoll` green; `go test -count=1 ./internal/dataentry/` green;
`just coverage-check` PASS at 78.2%.
- **ACL invariant verified:** `queryservice.go` holds no `store.Store` and
no plain `search.Searcher` — the only mentions of those types in the file are in
the doc comment explaining why it holds neither. Store reads still arrive via
the `Services` bundle through the unchanged `visibleListByTypes` /
`visibleEntitiesOfType` free functions, gated by the same `readGateFromContext`
scope.
- `isRelationLinked` confirmed dead in production: the only caller was
`TestIsRelationLinked`, a test exercising otherwise-unreachable code, so method
and test were deleted together.

**Deliberate deviation from the sibling pattern, and why it is correct:** the
sibling handlers hold fixed service handles BY VALUE, but this leaf holds
`visibleSearcher` and `affordances` as **closures**. Verified: tests reassign
`app.visibleSearcher` after construction (acl_search_test.go:251, 290, 304;
test_helpers_test.go) specifically to inject denying/recording searchers. A
by-value capture would have kept the construction-time searcher, so those ACL
tests would have exercised the wrong one and **passed vacuously**. The reasoning
is recorded in the type's doc comment. `schema`/`services` follow the sibling
pattern; a planned `gateRead` field was dropped since nothing rebinds it
(`readGateFromContext` is called directly, as the App method did).

## Quality

- [x] Code follows project patterns (leaf-service shape; deviation justified and documented in-code)
- [x] Checked for DRY opportunities — this IS the DRY step: next-action, scope and the read pipeline now share one collaborator instead of each threading its own closure later
- [x] No security issues introduced (ACL-scoped searcher preserved; no ungated search path introduced)
- [x] No silent failures
- [x] No debug code left behind
