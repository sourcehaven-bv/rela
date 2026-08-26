---
id: PLAN-6N61WY
type: planning-checklist
title: 'Planning: storetest: cover Freshness.LastModified and declare the Tx tier in Capabilities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN — a `RunFreshnessTests` suite wired into `RunAll`; a
`Capabilities.TxRollback` flag that runs the rollback suite from `RunAll`;
pgstore declaring it. OUT — fixing any backend. All three pass as-is; these
tests pin existing correct behavior so a *fourth* backend cannot get it wrong
silently.

**Acceptance Criteria:** see the ticket. Each maps to a subtest.

## Research

- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant rela concepts for prior art
- [x] ~~/research~~ (N/A: gaps identified by a targeted architecture survey)
- [x] ~~External libraries~~ (N/A: conformance tests over an in-tree interface)
- [x] ~~Reference implementations~~ (N/A)

**Existing Solutions:** the suite's own conventions — `Factory`, `seedEntities`,
`ctx()`, one exported `Run*Tests` per area, subtests named for the property.
`Capabilities.Attachments` is the precedent for gating an optional area.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** assertions are deliberately **coarse** — monotonicity
and coverage, not exact values. A backend whose timestamps come from the
filesystem or the database clock cannot promise more, and demanding equality
with a test-observed wall clock would fail those backends for no benefit.

`waitForClock` sleeps 10ms between writes because fsstore reads filesystem
mtimes, which have 1s granularity on some platforms; a sub-millisecond gap would
make "before" and "after" compare equal and the test flaky rather than
meaningful.

For the Tx tier, a `Capabilities` flag rather than always running the rollback
suite: fsstore and memstore omit the strong contract *deliberately* (they cannot
promise crash atomicity either), so running it unconditionally would fail two
correct backends.

Alternative rejected: asserting `LastModified` equals a captured `time.Now()`.
Fails fsstore on coarse-mtime filesystems and pgstore whenever the database
clock differs from the test host's.

**Files to modify:** `storetest/freshness.go` (new), `storetest/storetest.go`
(`Capabilities.TxRollback`, two `RunAll` entries), `pgstore/conformance_test.go`
(declare the tier, drop the separate entry point).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

Test-only change; no production code, no new input, no I/O beyond what the
existing conformance suite already performs against a temp dir or test schema.

Worth stating: `LastModified` is a **freshness signal, not an access-controlled
read**. It reports a timestamp across all entities and relations regardless of
principal, which is correct — it is consumed by index-rebuild logic, not served
to users — and these tests do not change that.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** empty store → zero time; entity write → advances; relation
write → advances (the one a new backend most often misses); reads → stable;
timestamp plausibility → catches a timezone-less parse. Integration coverage is
that all three real backends run the suite, pgstore against a live database.

**Edge Cases:** coarse filesystem mtime granularity (handled by `waitForClock`);
an empty store distinguishing "no data" from "zero"; reads advancing the
timestamp, which would make a consumer rebuild forever.

**Negative Tests:** the suite must FAIL for a backend that scans only entities —
verified by deliberate regression rather than assumed.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (s)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| New tests are vacuous and give false comfort | **Medium** | Verified by deliberate regression: dropping memstore's relation scan fails `CoversRelationWrites`. This is the exact failure mode found in TKT-415WA7's review, so it is checked rather than assumed |
| Flakiness from timestamp granularity | Medium | `waitForClock` at 10ms; assertions are monotonic (`!Before`, `After`) rather than equality |
| Gating rollback on a flag means a backend forgets the flag | Low | Strictly better than the status quo, where the whole suite was forgettable with no record of intent. A missing flag is now a visible absence in the `Capabilities` literal |
| Tests encode fs/pg assumptions a SQLite backend cannot meet | Low | Assertions are contract-level only; the UTC check is a plausibility bound, not a format requirement |

**Effort:** s.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

- [x] N/A — test-only change, no user-facing behavior. The rationale lives in
the suite's doc comments, where the next backend author will read it.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the two gaps were themselves the output of a
`go-architect` survey for pre-SQLite work, and each claim was independently
verified before implementing (`grep` for `LastModified` in storetest → zero
hits; `RunTxRollbackTests` call sites → exactly one). Two design decisions
carried from that review: keep assertions coarse enough for clock-driven
backends, and gate rather than force the rollback suite so fs/mem's deliberate
omission stays correct.
