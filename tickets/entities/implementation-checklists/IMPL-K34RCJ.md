---
id: IMPL-K34RCJ
type: implementation-checklist
title: 'Implementation: FuzzCloneNestedValues reports correct property-key refusals as crashes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The change is itself test code. The regression evidence is three committed fuzz
seeds, one per backend, each of which fails without the fix. The "integration"
dimension is the conformance suite: the same target runs against fsstore,
memstore and sqlitestore, so all three exercise it.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The oracle asserts against `ruleErr.Error()` from the validator rather than a
copied error string, which is the whole point of the change: the expectation is
derived from the contract, not restated beside it. Seeds hold only the property
name that matters. The store is built by the existing `FuzzFactory`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Load-bearing check — reverted the source change while keeping the seeds:

```
WITHOUT fix, WITH seeds:  --- FAIL: FuzzCloneNestedValues  (fsstore, memstore)
WITH fix:                 ok  fsstore / memstore / sqlitestore
```

Active fuzzing after the fix, 3 x 15s on fsstore plus 20s on memstore and
sqlitestore: all pass, no new crasher written to any corpus.

Full suite: `go test ./internal/store/...` — all packages pass, pgstore
included. Gates: `just arch-lint` OK, `just comment-lint` clean, `golangci-lint
run internal/store/storetest/...` 0 issues, `go build -tags sqlite ./...` clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: the change adopts an existing
pattern rather than adding one — it delegates to `storeutil.ValidateProperties`
instead of hand-modeling the rule, which removes duplication rather than
creating it)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows `FuzzPropertyValuesTypeZoo` and `createEntityOrSkip` in the same file,
including the pre-write `Clone()` so a backend that mutates the map it was
handed cannot move the goalposts.
