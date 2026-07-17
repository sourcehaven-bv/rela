---
id: IMPL-KEG5N
type: implementation-checklist
title: 'Implementation: Add Tx write-transaction contract to store.Store with fs/mem/pg implementations (DEC-8UIL0 phase 1)'
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
- `go build ./...` + `-tags postgres` + `-tags memorybackend`: all compile.
- `go test ./...` default tag: pass (incl. new `TestConformance/Tx` in fsstore
and memstore via storetest RunAll).
- `go test -race` on fsstore/memstore/storetest: pass — covers the
8-goroutine serialized read-modify-write counter (AC3) and view re-entrancy
(AC4).
- pgstore against a real PostgreSQL 15 (`rela_tx_test` db,
`go test -race -tags postgres ./internal/store/pgstore/...`): full suite pass;
verbose run confirms all five `TestConformance/Tx/*` subtests and all four
`TestTxRollback/*` subtests (CreateRolledBack, UpdateRolledBack,
NoEventsOnRollback, EventsDeliveredAfterCommit) executed, not skipped (AC1, AC2,
AC5).
- `just coverage-check`: package floor + total (76.2%) pass.
- `just plimsoll`, `just arch-lint`, `golangci-lint run ./internal/store/...`,
`gofmt -l`: clean.

**Stress hardening (added on review follow-up):**
- `TestTxCrossStoreSerialization` (pg): two independent pools/sessions on one
schema running concurrent Tx counters — proves the DEPLOYMENT-WIDE advisory-lock
serialization claim (the property no single-store test can see). Pass under
`-race`.
- `TestTxPoolExhaustion` (pg): 16 concurrent multi-write Txs on a MaxConns=2
pool — pins the no-second-connection property (the realistic pgstore deadlock);
asserts zero leaked write advisory locks in `pg_locks` and zero
idle-in-transaction sessions afterwards. Pass under `-race`.
- `storetest.RunTxStressTest` soak wired as `TestTxStress` in all three
backends: mixed workload (Tx counters, failing pair-Txs, nested Txs, plain
writers, readers, event subscriber) with a 5s no-progress watchdog that dumps
all goroutines (partial deadlocks never trigger Go's runtime detector). End
invariants: exact counter (no lost updates) and pair-atomicity (both-or-neither,
valid with and without rollback). 2s in CI; 30s local shakes run on all three
backends under `-race` (pg: ~1.4k serialized commits per 2s).
- Stress found one PRE-EXISTING fsstore race (list-vs-delete: lazy file load
during iteration can yield not-found for a concurrently deleted entity) —
independent of Tx, tolerated explicitly in the stress reader with a comment;
surfaced to the owner for a separate ticket.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
