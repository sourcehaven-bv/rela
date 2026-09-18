---
id: REV-33G7VB
type: review-checklist
title: 'Review: condition: predicate expressions on list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated checks

- [x] `just test` — `go test ./...` exit 0
- [x] `just lint` — exit 0; issue count identical to the pre-change baseline
(32 pre-existing advisory `prealloc`/`nolintlint` hints, none in the changed
files, verified by stashing and re-running)
- [x] `just arch-lint` — OK, no warnings
- [x] `just comment-lint` — no unresolvable doc links across 14,082 comments
- [x] `markdownlint` — 0 issues on both the docs source and the generated file
- [x] docs regeneration idempotent (`docs/` rebuilt from `docs-project/`,
second run produces no diff, so `docs-check` passes)
- [x] Frontend: 2458 tests pass, `vue-tsc --noEmit` clean, ESLint 0 errors
- [x] **pgstore conformance** under a live PostgreSQL — the parity suite is the
only enforcement of the backend-parity rule and SQL changed here, so a
memstore-only run would not have been evidence

## Code review

- [x] Reviewed
- [x] Findings addressed

**Review approach.** Three design findings were worked through with Jeroen
directly rather than via `/design-review`, and all three are closed in code:

1. *OR is all-or-nothing* — a constraint, not a defect. Encoded as the rule
that a disjunction with any unpushable arm pushes nothing.
2. *`~=` vs `PropNotEqual` disagree on unset rows* — a real defect, located in
the LOWERING rather than in `propmatch`. `propmatch` is shared with
`internal/filter`, CalDAV, feeds and the CLI, all of which are coherent today;
changing it would have altered four correct surfaces to fix a problem none of
them has. Closed by adding `PropNotEqualOrEmpty`.
3. *The authorization ceiling* — closed structurally, as distinct types, so the
widening append is a compile error rather than a review catch.

**Findings I raised against my own work and fixed:**

- An earlier wiring had `dataentry` importing `conditionlint`/`predicate`;
arch-lint refused. Reverted and re-done as consumer-side seams rather than
widening the rule.
- The generic `AdaptViewConditions` constraint immediately exposed that
`conditionlint` spelled the method `Match` where the seam wanted `Matches`.
Names aligned rather than keeping an adapter shim.
- My first draft documented `condition:` as available on kanbans. Nothing
evaluates it there, so it would have compiled at startup and then been silently
ignored — the exact failure class of this ticket. Changed to a config-load
**refusal** with a test naming what to delete later.
- I edited `docs/data-entry.md`, which is GENERATED from `docs-project/`.
Ported to the source and regenerated.
- Three lint findings fixed rather than suppressed: a broken doc link
(`nextActionMatchers` is unexported, so godoc cannot link it), a British
spelling, and a redundant lambda.

No `review-response` entities were created: the findings were resolved as they
arose, within the same session, rather than deferred.

## Acceptance verification

| Criterion | Result | Evidence |
|---|---|---|
| Disjunctive rule filters a list | **PASS** | `TestViewCondition_DisjunctiveRuleFiltersTheList` drives the real HTTP handler with the production compiler |
| `condition:` ANDed with `filters:` | **PASS** | composed via `predicatefns.AndFilters`; neither weakens the other |
| Non-compiling condition is a load error naming the view | **PASS** | `TestCompileViewConditions` (9 cases incl. the `!=` dialect trap) |
| Paging and counts describe the filtered population | **PASS** | `TestViewCondition_TotalMatchesTheFilteredPopulation` |
| No pushdown regression for the conjunctive case | **PASS** | existing `listpushdown` tests unchanged; guard is condition-scoped |
| Backend parity for the new store ops | **PASS** | `storetest` on memstore, fsstore, sqlitestore, pgstore |
| Store primitives are load-bearing | **PASS** | each mutation-tested; see the implementation checklist |
| Docs state the three verified traps | **PASS** | every sample executed against the live engine |

**Deferred to [[TKT-LPLZ1V]]:** kanban read path, disjunctive SQL pushdown,
ordered-`PropOp` constant folding, the hash rename, field-to-field comparison,
and refusing `rrule_next` in favour of `computed:`. Each is named in that
ticket's implementation sequence with its own rationale.
