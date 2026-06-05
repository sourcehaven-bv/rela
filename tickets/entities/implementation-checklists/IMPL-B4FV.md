---
id: IMPL-B4FV
type: implementation-checklist
title: 'Implementation: store: generic GraphQuery DSL + naive impl + storetest conformance'
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

### Acceptance verification

| AC | Verdict | Evidence |
|---|---|---|
| AC1 (all 3 backends compile) | PASS | `just build-check-tags` — fsstore default, memorybackend tag, postgres tag all compile |
| AC2 (conformance passes on all 3) | PASS | `go test ./internal/store/...` — fsstore, memstore, pgstore all green; same RunGraphQueryTests subtests run via RunAll |
| AC3 (GraphCount matched + total) | PASS | `GraphCount_matched_and_total` subtest |
| AC4 (CI gates clean) | PASS | `just ci` clean — lint, fmt, arch-lint, full tests + race, docs all green |
| AC5 (real postgres) | PASS | `RELA_TEST_DATABASE_URL=postgres://jeroen@/rela_test_pr1?host=/tmp&sslmode=disable just test-postgres` — pgstore tests green against real postgres |

### Files added

- `internal/store/graphquery.go` (65 LOC) — types, RelationPredicate, GraphQueryer interface
- `internal/store/graphquerynaive/naive.go` (~215 LOC) — generic iterate-and-filter; DepthCap=5
- `internal/store/fsstore/graphquery.go` (~22 LOC) — wrapper
- `internal/store/memstore/graphquery.go` (~21 LOC) — wrapper
- `internal/store/pgstore/graphquery.go` (~24 LOC) — wrapper with follow-up flagged
- `internal/store/storetest/graphquery.go` (~280 LOC) — conformance suite, 10 subtests

### Files modified

- `internal/store/store.go` — embed `GraphQueryer` into `Store` interface
- `internal/store/storetest/storetest.go` — wire `RunGraphQueryTests` into `RunAll`
- `.go-arch-lint.yml` — declare `graphquerynaive` component + per-backend mayDependOn

### Manual end-to-end

- Tested with all three backend compile-tags via `just build-check-tags`.
- Ran the full `RunAll` suite against an in-memory memstore and a tmpdir fsstore to confirm identical results.
- Ran `just test-postgres` against a local postgres to confirm pgstore delegation works against real pgx.

## Quality

- [x] Code follows project patterns (check similar code) — DSL mirrors existing `EntityQuery` / `RelationQuery` shape
- [x] Checked for DRY opportunities — the single naive impl shared by all three backends IS the DRY win
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned) — iterator yields `(nil, err)` per project convention
- [x] No debug code left behind
