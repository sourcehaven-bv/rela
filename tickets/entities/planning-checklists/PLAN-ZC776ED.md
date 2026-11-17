---
id: PLAN-ZC776ED
type: planning-checklist
title: 'Planning: Store query contract: or, not and ordered comparisons'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** See TKT-8HY05WB "Scope". In: `GraphQuery.Where` boolean tree, typed
ordered ops, opt-in nil semantics, every backend, LowerCondition and its
consumers. Out: dates (TKT-4Z2BIRG), refusing unlowerable conditions
(TKT-W0DYCEC), hidden-field rewrite (follow-up), removing the Go fallbacks
(TKT-9BKVQ9I, TKT-I3MGCRT).

**Acceptance Criteria:**
1. Every backend answers And/Or/Not, ordered string and integer leaves and relation leaves the same: `storetest.RunGraphCondTests` and `RunGraphDifferential`.
2. LowerCondition lowers or, not and ordered comparisons: `TestLowerCondition_Trees`, declines in `TestLowerCondition_TreeDeclines`.
3. Lowered answers equal the Go evaluator: `TestLowerCondition_AgreesWithEvaluator`.
4. Query scopes and MCP push or/not with gated traversal leaves: `appbuild` queryconditions tests.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: extends existing GraphQuery and LowerCondition; the design is in the ticket)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: SQL boolean rendering is standard; the work is rela's own contract)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** No library: the tree renders onto the existing
condBuilder paths. Reused `matchesProp`/`matchesPredicate` (graphquerynaive),
`buildPredicateSQL` (pgstore), the sqlite unsafe fallback,
`ConditionConjunction` leaf rules, and `relresolve` hop gating.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See TKT-8HY05WB "Design".

Rejected alternatives:
- a global nil-is-false change, because it widens ACL and automations (RR-R15R2DT);
- a separate Or field next to Props, because callers copying only Props would widen (RR-I92MF14).

**Files to modify:** `internal/store/graphquery.go`, graphquerynaive, pgstore,
sqlitestore, storetest, `internal/predicate`, `internal/predicatefns`,
`internal/propmatch`, `internal/queryplan`, `internal/visibility/entitypage.go`,
`internal/appbuild`, `internal/mcp`, `internal/dataentry/listpushdown.go`,
`internal/conditionlint`, `internal/affordances/bindings.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Conditions come from config and from MCP
callers. Only shapes `ConditionTree` allowlists lower; `ValidateCond` rejects
bad kinds, values, depth, width and `Negate` with `ErrInvalidQuery`. Values are
bound as SQL parameters.

**Security-Sensitive Operations:** Relation leaves are gated per reader
(`visibility.GateCond`); a denied hop is false. Pushdown needs every read field
visible on every row. ACL keeps fail-closed nil semantics.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** As in the acceptance criteria above.

**Edge Cases:** absent key, JSON null, empty string, lists, objects, booleans,
"+3", "007", " 7", 2.5, 1e19, 2^63, negative numbers, NOT over absent keys,
empty And and Or.

**Negative Tests:** invalid integer value, Kind on a non-ordered op, Not arity,
over-deep and over-wide trees, `Negate` in a tree, `Where` in rollup gates and
in `pageViaFilter`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Go/SQL disagreement on odd values: mitigated by the two
differentials. Widening through partial copies: one return type and
`store.AndWhere`. Effort: l.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: no config syntax changes; conditions that already worked are now pushed to the store)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing docs change)

**Documentation Impact:** N/A, internal change.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-I92MF14, RR-R15R2DT, RR-5S8QI60, RR-RDJW4VU,
RR-8JE0G2Q, RR-IEZT6ZA, RR-V8E4IDV, RR-AZSV267
