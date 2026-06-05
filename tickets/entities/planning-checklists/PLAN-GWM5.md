---
id: PLAN-GWM5
type: planning-checklist
title: 'Planning: store: generic GraphQuery DSL + naive impl + storetest conformance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** documented in the ticket body (see TKT-ZYH3). In summary:
add `store.GraphQuery` / `GraphCount` + naive backend-agnostic
implementation + storetest conformance. Out: any push-down, any
consumer.

**Acceptance Criteria:** AC1–AC5 in the ticket body. Each maps to
a specific test in `internal/store/storetest/graphquery.go` or to a
CI gate (lint, arch-lint, just ci, just test-postgres).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small,
self-contained DSL; design already validated on the reference
branch `feat/acl-v1-tkt-svxl` and in `.ignored/acl-prototype/`)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. The reference branch `feat/acl-v1-tkt-svxl`
contains the full design context. The shape was validated against
a Python prototype before the Go implementation.

**Existing Solutions:**

- Looked at general-purpose graph-query libraries (GraphQL-shaped,
Cypher-shaped) — rejected as misfits for a single-method store
extension; the bespoke DSL fits the existing iterator-returning
store contract.
- Same shape as `store.EntityQuery` / `store.RelationQuery` already
in the package — the new types fit naturally alongside.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. New types in `internal/store/graphquery.go` + interface embedded
into `Store`.
2. Backend-agnostic naive impl in `internal/store/graphquerynaive/`:
BFS expand for both endpoint and entity sides, visited-set
termination, depth-cap backstop, iterate-and-filter the entity-type
candidates.
3. Per-backend wrappers that just delegate to the naive impl. This
is the structural defense against backends drifting in behavior —
all three exercise exactly the same code path via the conformance
suite.
4. Wire `RunGraphQueryTests` into `RunAll` so every existing and
future store impl runs the conformance suite for free.

**Files to modify:**

- `internal/store/graphquery.go` (new)
- `internal/store/graphquerynaive/naive.go` (new)
- `internal/store/{fsstore,memstore,pgstore}/graphquery.go` (new)
- `internal/store/store.go` (embed `GraphQueryer`)
- `internal/store/storetest/graphquery.go` (new conformance)
- `internal/store/storetest/storetest.go` (wire into `RunAll`)
- `.go-arch-lint.yml` (new component + mayDependOn)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None. `GraphQuery` consumers are
internal Go callers; no validation needed at this layer. The future
ACL consumer (out of scope) will be the boundary that decides what
queries are safe to construct.

**Security-Sensitive Operations:** None directly. The store's
existing access patterns (which the new method composes against)
are unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `RunGraphQueryTests` subtests cover:

- `HasInbound_direct` — no transitive expansion
- `HasOutbound_direct` — outbound symmetry
- `InheritThrough_endpoint_expansion` — groups-shaped
- `EntityInheritThrough_entity_expansion` — containment-shaped
- `Both_expansions_compose` — independence
- `OfTypes_filter` — type-filtered
- `SelfLoop_terminates` — cycle safety
- `Cycle_terminates` — multi-node cycle safety
- `Depth_zero_is_no_op` — no expansion when Depth=0
- `GraphCount_matched_and_total` — matched + total semantics

Each runs against fsstore, memstore, pgstore via `RunAll`. pgstore
additionally runs against real postgres via `just test-postgres`.

**Edge Cases:** empty endpoint set (returns nil), Depth=0 with
non-empty InheritThrough (no expansion), Depth>DepthCap (truncated
at cap), HasInbound and HasOutbound both nil (every entity of type
matches), missing OfTypes (all relation types match).

**Negative Tests:** none specific — the naive impl propagates store
errors via the iterator's `(nil, err)` shape; that's already
covered by the broader storetest harness.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **R1 (mitigated):** pgstore would have been quadratic if it kept the
naive delegation. **Mitigated** by shipping a SQL-native recursive-CTE
impl in this same PR (one round-trip per call). The conformance suite
in `RunAll` is what makes this safe — the SQL impl is verified
behavior-equivalent to the naive impl that fsstore/memstore use.
- **R2 (mitigated):** Recursive CTE plan stability. **Mitigated** by
migration 0002 (composite (rel_type, from_id|to_id) indexes so the
CTE planner has a fast path for the per-iteration rel_type filter).
- **R3 (low):** Tracer overhead in production. **Mitigated** by the
`debugEnabled` check at Open time — production deployments running
slog at Info or Warn skip tracer attachment entirely, paying zero
per-query overhead.

**Effort:** M (was S before adding SQL-native pgstore + tracer in
this PR's scope). Two days net — the SQL builder, migration,
benchmarks, and tracer added ~1 day of focused work on top of the
straight port.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal store API only)

**Documentation Impact:**

- [ ] User guide / reference docs — N/A: internal store API
- [ ] CLI help text — N/A
- [ ] CLAUDE.md — could mention the DSL in the store backends rule,
but adding it now would be premature (no consumer); revisit when
the first consumer lands.
- [ ] README.md — N/A
- [ ] API docs — godoc on the new types is the API doc
- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A:
design already reviewed on the reference branch
`feat/acl-v1-tkt-svxl` by the cranky-code-reviewer agent across
two passes; this PR ports the agreed shape)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Reference-branch RR mapping for this PR
(per `.ignored/acl-v1-split-plan.md`): none of the 22 RRs apply
directly to this PR's scope. The DSL itself was not the source of
any finding — RRs landed on consumers (acl resolver, affordances,
appbuild wiring).
