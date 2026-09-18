---
id: PLAN-5ZGS9T
type: planning-checklist
title: 'Planning: Query-budget test for the nested view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope — test-only, no production behaviour change:

- A `storetest.Counting` budget test for a `display: nested` view section,
  asserting the store-read count is identical at 10 and 50 parent rows.
- A pinned budget constant beside the four existing ones, with the per-read
  breakdown in its doc comment.
- A direct assertion that selection happens BEFORE relation-column resolution
  (the RR-HKHPYG invariant).
- The fixture additions needed to host a two-level nested section in
  `budgetMeta`/`budgetConfig`.

OUT of scope:

- Any change to `sections_nested.go` or other production code. If the test
  reveals a cost defect, that is a new ticket — this one pins today's cost.
- Lowering `nestedNodeBudget` or making it configurable.
- Sorting (TKT-9OFGH4) and rollup (TKT-ZAD9PS).

**Acceptance Criteria:**

1. **Size independence.** A budget test drives a `_views` request whose view has
   a `display: nested` section, at 10 and 50 parents, and `assertBudget` reports
   equal read counts.
2. **Pinned constant.** The measured count is a named constant next to
   `listPageBudget`/`viewSectionBudget`/`searchBudget`/`recursiveViewBudget`,
   documented per-read.
3. **RR-HKHPYG regression guard.** A test fails if relation-column resolution is
   moved back ahead of the budget.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: effort `s`, a test against an existing harness)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, and the pattern is already established in-repo.

**Existing Solutions:**

No library question: this is an in-repo harness. The whole apparatus exists
already in `internal/dataentry/querybudget_test.go` (268 lines, TKT-1U8XYN):

- `newBudgetApp` (`:103`) — seeds n tickets into a `storetest.NewCounting`
  wrapper over a memstore BEFORE the app is assembled, wires an ACL reached
  through a `member-of` walk, then `Reset()`s the counters.
- `readsFor` (`:157`) — runs an op at `[]int{10, 50}` and returns both counts.
- `assertBudget` (`:170`) — fails when the two differ, and separately when the
  large count differs from the pin.

Four tests use it: list page (6), view table section (11), search (3),
recursive view traversal (12). `TestQueryBudget_ViewTableSectionIsSizeIndependent`
(`:207`) is the closest model — a view whose table section carries two relation
columns — because a nested section resolves relation columns through the very
same `resolveRelationColumns`.

**Prior art consulted:**

- `internal/dataentry/sections_nested.go` — `buildNestedTree` (`:56`),
  `planNestedRows` (`:227`), `nestedColumnUnion` (`:132`), `indexed` (`:175`).
- `internal/dataentry/views_handler.go:848` — `resolveRelationColumns` and
  `relationColumnTargets` (`:874`).
- `internal/store/storetest/counting.go` — what the harness can and cannot see.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Two tests, because the two acceptance criteria need different instruments.

**(a) The budget test — AC1 + AC2.** Extend the existing fixture and add a
fifth `TestQueryBudget_*` in the established shape:

1. `budgetMeta`: add a `program` entity type and TWO relations —
   `in-program` (`epic → program`) and `in-epic` (`ticket → epic`), both
   traversed with `follow_incoming`. Two are needed, not one: a `ticket → epic`
   edge alone cannot get a program-entry view to its epics. Both are
   single-target because `validateLevelColumns` resolves the child type through
   `determineTargetType`, which returns `""` for a multi-`to:` relation (which
   is why the existing `tracked-by` is unsuitable).
2. `budgetConfig`: add a view whose entry type is `program`, with two traverse
   steps (program → epics, epics → tickets) and one `display: nested` section
   carrying `parent_columns` and `child_columns`, the child level including a
   **relation** column so `resolveRelationColumns` is genuinely exercised.
3. `newBudgetApp`: seed one program and one epic per `budgetTicketsPerEpic`
   tickets, so BOTH levels grow with n. Holding the parent level at a single
   row would hide a per-parent N+1, which is the regression the ticket is
   actually about.
4. The test calls `viewsAs(ctx, t, app, d, "program", "PRG1")`, asserts 200,
   and passes the count to `assertBudget` against a new `nestedSectionBudget`.

**Entry type must be fresh.** `findViewByEntityType`
(`internal/dataentry/default_view.go:12`) iterates the views MAP and returns the
first entry whose `Entry.Type` matches, so two views sharing an entry type make
BOTH pick nondeterministically per run — a bug `-shuffle=on` finds and a local
run does not. The recursive view's own comment records this hazard after it bit
someone. `ticket` and `epic` are taken, hence `program`.

**(b) The regression guard — AC3.** A new `storetest.Breadth` store decorator
that records how many IDS each batched read was handed, and a test asserting a
nested section's relation-column queries stay proportional to the EMITTED tree
rather than the visible one.

*This replaces the original plan, which was wrong.* The first version proposed a
unit test on `planNestedRows` asserting the plan is bounded by
`nestedNodeBudget`. Design review disproved it empirically: with the RR-HKHPYG
defect reintroduced, that test PASSES. `planNestedRows` is a pure function and
cannot observe what its caller does with the returned plan — the defect was
never in the planner but in `buildNestedTree` choosing to pass
`result.Collections[sec.Children]` to the resolver instead. The assertion was
also a near-duplicate of `TestNestedSection_SectionBudgetDropsParentsAndFlags`.

The working instrument records `len(EntityQuery.IDs)` and
`len(RelationQuery.EntityIDs)`. `Counting` answers "how many round-trips",
`Breadth` answers "how wide was each one"; an N+1 moves the first, an
over-fetch moves only the second. Verified by mutation in both directions:
with the defect `ListRelations` carries 8,911 ids, without it ~2,400, against a
`nestedRelationIDLimit` of 4,000.

Two further corrections found while building it:

- The obvious assertion target (`ListEntityHeaders`) is the WRONG one.
  `visibleTitles` only takes the header path when the view reader implements
  `visibility.HeaderFilterer` and otherwise falls back to `ListEntities`; in
  this fixture it takes the fallback, so that method saw 1 id. `ListRelations`
  is the read the defect actually widens.
- `ListEntities` must be EXCLUDED from the assertion. The view pipeline loads
  each collection in full before any section builder runs, so it carries the
  whole visible universe by design and is byte-identical with and without the
  defect. Including it made the assertion untrippable.

The fixture derives its size from `nestedRelationIDLimit` and
`nestedChildPreview` rather than hardcoding a parent count, and fails loudly if
the visible universe cannot exceed the limit — because a fixture too small to
trip the guard passes whether the bug is present or not, which happened at an
intermediate size.

**Files to modify:**

- `internal/dataentry/querybudget_test.go` — fixture additions, the size-independence
  test, the breadth test, the fixture-validity test, two new constants.
- `internal/store/storetest/breadth.go` — the new `Breadth` decorator (NEW file).

No production file changes. `internal/store/storetest` is test infrastructure,
not shipped code, so the test-only scope holds; the user approved adding a
purpose-built wrapper there rather than extending `Counting`, which keeps the
four existing pins structurally unable to move.

**Alternatives considered:**

- *Assert the RR-HKHPYG regression via the read count.* Rejected: it cannot
  work. `relationColumnTargets` issues one `ListRelations` per (column, row
  type) and passes the ids as `RelationQuery.EntityIDs`, so resolving over
  280,000 ids costs the same NUMBER of calls as resolving over 2,000. The defect
  is in argument size, which `storetest.Counting` does not record — it counts
  method hits only (`counting.go:37`). A count-based guard here would be a test
  that passes for the wrong reason.
- *Extend `Counting` to record argument sizes.* Rejected, but not for the
  original reason (that a unit test would do it better — it would not). A
  separate decorator keeps the two measurements separable: `Counting`'s doc is
  explicit that it counts calls because "an N+1 in a handler is N store calls
  whatever the backend", and breadth is a different question. Additive changes
  to `Counting` would also have been safe, but a sibling cannot perturb the four
  existing pins even in principle.
- *Reuse the `epic` entry type.* Rejected — the nondeterminism above.
- *Drive `buildNestedTree` directly instead of through HTTP.* Rejected: the
  existing four tests all go through the real handler, and the point of a budget
  test is to measure the whole request path including the ACL membership walk.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

None new. This is a test-only change; all inputs are fixture constants authored
in the test file. No user input, no config parsing, no external API.

**Security-Sensitive Operations:**

None introduced. The test exercises an ACL-gated read path, and the fixture
grants its principal `read: ["*"]` so gating is not what is under measurement —
the ACL behaviour of `display: nested` is already covered by
`TestNestedSection_HiddenChildIsAbsent` and the security review of TKT-MJKZQ3.

One thing worth stating because it is easy to get backwards: a budget test must
not be read as evidence about the gate. It measures cost, not confidentiality.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | How it is verified |
| --- | --- | --- |
| 1 | `TestQueryBudget_NestedSectionIsSizeIndependent` | `readsFor` at n=10 and n=50; parents scale 2→10 AND children 10→50, reads flat at 15 |
| 2 | same | `assertBudget`'s second arm compares against `nestedSectionBudget = 15` |
| 3 | `TestQueryBudget_NestedRelationColumnsResolveOverEmittedRowsOnly` | `storetest.Breadth` records ids per read; mutation-verified (defect 8,911 ids / fixed ~2,400, limit 4,000) |
| — | `TestQueryBudget_FixtureConfigIsValid` | not an AC; closes the RR-S3 gap that the fixture was never validated |

**Integration vs unit:** (a) is an integration test — it goes through the real
HTTP handler, the real ACL gate, and the real view pipeline. (b) is a unit test
on the selection function, which is the right granularity for an invariant about
one function's output.

**Edge cases:**

- *Zero parents* — `buildNestedTree` returns `(nil, false)` early; already
  covered by `TestNestedSection_EmptyParentsIsEmptySection`.
- *A parent with no children* — already covered by
  `TestNestedSection_CappedParentIsDistinguishableFromChildless`.
- *A fixture too small to trip the guard* — the failure mode that bit twice
  here. The breadth test asserts its own adequacy (`visibleChildren >
  nestedRelationIDLimit`) and derives its size from the constants, so it
  cannot silently degrade into a test that passes regardless.
- *`nestedChildPreview` vs `nestedNodeBudget`* — both bounds apply per parent
  simultaneously, they do not bind in sequence: `min(len(kids),
  nestedChildPreview, budget)` caps each parent at 25 children WHILE the
  running budget drains, so 2000 nodes is reached after 77 parents and the
  rest are dropped whole.

**Negative Tests:**

The pin itself is the negative test: any change that adds a per-row store read
makes `small != large` and names itself in the failure message via
`counting.String()`. The AC3 test is negative in the same sense — it fails on
the specific regression it is named for.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
| --- | --- |
| Adding a view to `budgetConfig` perturbs the four existing pinned budgets | Run the whole `TestQueryBudget_*` set before and after; a fresh entry type means `findViewByEntityType` cannot re-route an existing test. If a pin does move, investigate rather than re-pin. |
| Fixture changes alter the seeded graph the other tests measure | New entities/relations are additive and hang off a new `program` root; existing tests read `TKT-0001` and `E1`, whose edges are unchanged. Verify by diffing the pins. |
| The measured count turns out to grow with n | That is the test doing its job — it means a real defect exists. Per scope, fixing it is a separate ticket; this one would then land the failing-test evidence and the ticket. |
| `-shuffle=on` nondeterminism from a duplicate entry type | Fresh `program` type; documented in the fixture comment. |

**Effort:** s — confirmed. Two tests against an existing harness.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: test-only, no user-visible surface)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: `kind: test`, nothing user-facing changes)

**Documentation Impact:** N/A — internal test. No `docs/` page describes query
budgets; the convention lives in root `CLAUDE.md`, which already states the rule
this ticket satisfies and needs no amendment.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Run against the plan before implementation. Verdict was "do not implement as
written"; all findings addressed.

| ID | Severity | Status | Finding |
| --- | --- | --- | --- |
| RR-C1 | critical | addressed | AC3 test was a no-op guard — passes with the defect live, and duplicates an existing test. Replaced with the `Breadth` decorator; mutation-verified in both directions. |
| RR-S2 | significant | addressed | Plan named one relation (`part-of: ticket → epic`) for a traverse needing two. Fixture uses `in-program` (epic→program) and `in-epic` (ticket→epic). |
| RR-S3 | significant | addressed | `newAppFromParts` never calls `ValidateConfig`, so a budget could be pinned for a config no operator could load. Added `TestQueryBudget_FixtureConfigIsValid`, mutation-verified. |
| RR-S4 | significant | addressed | Fixture held the parent level constant at 1 row, so a per-parent N+1 — the exact thing the ticket asks about — would be invisible. Parents now scale 2→10 as children scale 10→50. |
| RR-M5 | minor | addressed | Confirmed the four existing pins are unperturbed; nested measures 15. |
| RR-M6 | minor | addressed | Plan's arithmetic was right, its reasoning wrong: both bounds apply per parent simultaneously (77 parents × 25 children = 2000), they do not bind in sequence. Test comments corrected. |
| RR-M7 | minor | wont-fix | Asserting the CAUSE of `truncated` rather than the boolean — moot, since the test that asserted `truncated` was removed with RR-C1. |
| RR-L9 | minor | addressed | Noted in the test comment that the pin is measured ungated, so the number is not a worst case. |
| RR-L10 | minor | no action | Coverage floors not at risk; tests only move coverage up. |
