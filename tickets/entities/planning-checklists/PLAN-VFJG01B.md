---
id: PLAN-VFJG01B
type: planning-checklist
title: 'Planning: Roll-ups along a chain, over ordered siblings and in list conditions'
started: "2026-11-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: `along:` (exists/min/max, depth 1 to 32), `siblings_before`, one level of roll-up over roll-up, roll-ups in list `condition:` (pg LATERAL leaf, Go path elsewhere), `refresh_types`, the omnifocus example. Out: `all`/`count` along a chain, descendant walks, deeper nesting, defer dates on the SQL path (TKT-4Z2BIRG), roll-ups in `visible:` `when:` (refused at load).

**Acceptance Criteria:**
1. A deferred or on-hold ancestor holds an action on every backend (storetest along conformance and differential).
2. Sequential blocking follows `_order_in` after a reorder (storetest siblings, CompareOrderKeys parity).
3. A project with no available action is stalled (nested roll-up tests; omnifocus example).
4. A list condition on a roll-up returns the same rows on every backend and pushes down on pg (leaf differential and EXPLAIN tests).
5. Hidden rows never change a roll-up answer or cap outcome (ACL cap tests with a real acl.yaml).
6. Invalid shapes fail the load (metamodel and rollup refusal tests).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design doc covers the research)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the design doc (`design-gaps.md`, revision 3 plus section 11 deviations) in the omnifocus probe workspace holds the research.

**Existing Solutions:**
Builds on the existing `via:` roll-ups (internal/rollup, visibility.Rollups), relresolve hops, queryplan lowering and the pg keyset scan. OmniFocus availability rules were the reference. No library fits: the walk must be gated per reader inside the store.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Per the design doc (`design-gaps.md`, revision 3 plus section 11 deviations) in the omnifocus probe workspace: unrolled seeded gated hop CTEs with earliest-arrival dedupe, a per-source cap on gated rows plus HopCap on raw reads, a guarded `_order_in` cast, a load-time dependency graph for nesting, and a per-candidate LATERAL leaf in the keyset scan. Alternatives rejected: recursive CTEs (gating and dedupe per level are not expressible), Lua materialization (stale on reorder), stored derived columns (per-reader values).

**Files to modify:**
internal/metamodel, internal/rollup, internal/store (+ pgstore, sqlitestore, memstore, graphquerynaive, storetest), internal/visibility, internal/queryplan, internal/dataentry, internal/apiwire/v1, frontend list refresh, docs-project guides, examples/omnifocus.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
Only config (schema.yaml roll-up definitions, list conditions). Every shape is validated at load with an allowlist of options per roll-up kind; invalid shapes fail the load. Names reach SQL only as bind parameters.

**Security-Sensitive Operations:**
Reads across related entities: every node, sibling and inner value is gated per reader before the fold; caps and timeouts fail closed per source; absent values fail closed under every comparison.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
See acceptance criteria 1 to 6; each maps to the named storetest, visibility, dataentry and metamodel tests, run on memstore, sqlite and pg.

**Edge Cases:**
Cycles in data, depth cap, hub fan-out over HopCap, cap hit, hidden ancestors and siblings, faced entities, extreme order keys (±1e400, ±1e-400), unset defaults, keyset pages after the first.

**Negative Tests:**
Invalid shapes fail the load with a clear error; over-cap, timeout and withheld values make the roll-up absent and drop the row in conditions.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Cost on large graphs: per-level caps, HopCap, statement timeouts, EXPLAIN budget tests. Leaks through caps: per-source gated cap. Residual: ungated hop reads affect latency (fails closed), recorded in the threat model.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
docs/metamodel.md and docs/acl-security.md via their docs-project guides; examples/omnifocus/README.md.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Two design-review rounds on the design doc (`design-gaps.md`, revision 3 plus section 11 deviations) in the omnifocus probe workspace; their findings were resolved in revision 3 and are recorded there, not as RR entities.
