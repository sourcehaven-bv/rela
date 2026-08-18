<!-- @managed: claude-workflow v1 -->
---
id: PLAN-ERHWL0
type: planning-checklist
title: 'Planning: Memoize dashboard breakdown and table-row derivation'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `getBreakdown` / `getTableRows` in `frontend/src/views/DashboardView.vue`
become memoized derivations, evaluated once per card per render instead of once
per template call site (each is referenced twice).

OUT: everything that changes the wire format or the amount of data fetched.
Server-side aggregation (TKT-AIEGHU), limit/sort pushdown (TKT-8AUD1U) and
bounding the structured search path (TKT-T0DK37) are separate tickets under the
same feature — deliberately so, because each collides with in-flight work
elsewhere while this one does not.

**Acceptance Criteria:**

1. `getBreakdown` evaluates at most once per card per render.
   Test: a `status` property getter that counts reads; 3 entities through two
   template call sites must produce 3 reads, not 6.
2. `getTableRows` evaluates at most once per card per render.
   Test: a sorted table card renders rows in the configured order.
3. No change to rendered output for count, breakdown or table cards.
   Test: the 7 pre-existing `DashboardView.test.ts` cases (from #1316) stay green.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — xs refactor, single file, no approach ambiguity.

**Existing Solutions:**

- No library needed: Vue's own `computed` is the memoization primitive.
- Prior art in-tree: `frontend/CLAUDE.md` already documents this exact hazard for
  dense surfaces — "Resolve per column/field, not per cell — `resolve()` walks a
  Map and can `console.warn`, which at 200 rows is 200 warnings per render. See
  `PropertyDisplay.vue`'s precomputed `rows` (RR-UD2A) for the pattern." This
  ticket applies the same fix one surface over.
- The card-identity problem was already solved upstream: PR #1316 (TKT-53KICM)
  introduced `cardKey()` because the card list is per-principal and an
  index-keyed map binds one card's data to another's tile. Reused as the memo
  key rather than inventing a second keying scheme.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Two `computed` maps, `breakdowns` and `tableRows`, each keyed by `cardKey(card)`
and built by iterating `cards.value`. The exported `getBreakdown` /
`getTableRows` become thin lookups into those maps, so every template call site
is unchanged and the diff stays confined to the `<script setup>` block.

Alternatives rejected:

- **Per-card child component.** Vue would memoize naturally, but it restructures
  the template for a perf fix and conflicts far harder with #1316.
- **Cache in a plain `Map` keyed by card, invalidated manually.** Reintroduces
  the invalidation bug `computed` exists to prevent.
- **Compute inside `loadData` and store the result.** Couples derivation to
  fetching; a later config-only change (e.g. `group_by`) would not re-derive.

**Files to modify:**

- `frontend/src/views/DashboardView.vue`
- `frontend/src/views/DashboardView.test.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new inputs. The derivations read the same two sources as before: card config
from `/_config` and entity rows from `/_search`. Both already flow through the
ACL-scoped read path server-side; this change alters only *when* the client
reduces them, never *what* it receives.

**Security-Sensitive Operations:**

None. Specifically NOT a place to add filtering: #1316 pins that the view must
render whatever the server returned and must not re-decide visibility
client-side (`does not filter on card.permission client-side`). Memoization
preserves that — it caches the reduction, it does not gate it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
|---|---|
| AC1 breakdown derived once | counting getter on `status`; expect reads == entity count |
| AC2 table rows derived once | sorted card renders `['a','b','c']` from input `['c','a','b']` |
| AC3 no output change | the 7 existing #1316 cases stay green |

**Edge Cases:**

- Card with no `group_by` → empty breakdown, no crash.
- Card key absent from `cardData` (search still in flight) → `|| []` fallback.
- Data arriving *after* first render — the memo must invalidate. `cardData` is a
  `ref` holding a `Map` mutated via `.set()`, so this is the real reactivity
  risk; covered by resolving the search promise post-mount.

**Negative Tests:**

Mutation test: revert the getter to its non-memoized form and confirm the
counting test fails (6 reads vs 3). A guard that does not fail against the old
code pins nothing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Memo fails to invalidate when card data lands.* The plausible failure mode,
  given `.set()` mutation on a Map inside a `ref`. Mitigated by an explicit test
  that renders before the search resolves, then asserts the value appears.
- *Conflict with #1316.* It rewrites the same getters. Mitigated by branching
  from it rather than develop; since merged, rebased onto develop cleanly.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

N/A — internal refactor. No user-visible behaviour, config surface or API
changes, so no docs-checklist (`kind: refactor`, so the
`done-enhancement-needs-docs-done` gate does not apply).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — xs single-file refactor with no interface or
wire-format change. The one design decision (reuse `cardKey` rather than add a
second keying scheme) was settled by reading #1316.
