---
type: planning-checklist
title: 'Planning: condition: expressions on list/kanban/feed/CalDAV views — boolean
  composition, current_user, and disjunctive pushdown'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — one `condition:` key holding a predicate expression, on lists, kanbans,
feeds and CalDAV collections; compiled at config load; ANDed with the surface's
existing `filters:`/`where:`; boolean composition (`or`/`not`/grouping);
`current_user` + `is_current_user`/`has_current_user`; a Go read path for
kanbans; per-principal cache keying and fail-closed on an unidentified request;
disjunctive pushdown into `store.GraphQuery`; docs.

OUT — per-column kanban filters; `applyV1Filters` convergence (TKT-UTJ24Z);
temporal-parsing consolidation (TKT-HFEKVN); `condition:` as a user-editable or
URL filter; boolean composition inside `internal/filter`; ordered/computed
comparison in `store.PropOp`.

**Scope decision taken:** this ticket ABSORBS TKT-ZQV9O5 (user decision). Two
`condition:` keys with different semantics on one config surface would be the
bad outcome. TKT-ZQV9O5 closes as superseded on acceptance. Its design is
already anticipated in code — `internal/appbuild/nextaction_matchers.go:115`
names "the boundary stamp (TKT-ZQV9O5)" as the stricter shape to adopt.

**Acceptance Criteria:** the 13 criteria on TKT-LPLZ1V. Criterion 1's fixtures
are the six verified rows in the ticket's "Verified engine behaviour" table.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-6PK0S3 (linked via `has-research`). No new `/research`
run: that doc already settles the governing question — filter = data-shaping,
predicate = policy/conditions — and root `CLAUDE.md` has since recorded the
boundary as settled architecture. A second survey would re-litigate a decision
already made.

**Existing Solutions:**

No external library is in question: `internal/predicate` already IS the
expression engine, with `and`/`or`/`not`/parentheses, typed compilation, and
fixed depth/step budgets. This ticket adds a config surface, not a language.

Reusable in-tree, all verified during planning:

- **Next-action `condition:` — the closest analogue and the model to copy.**
  `query:` selects, `condition:` refines
  (`docs/data-entry.md:1809`, `:1822`); compiled at load by
  `conditionlint.CompileNextActions` (`internal/conditionlint/nextaction.go:44`);
  partly pushed down via the `ConditionPrefilterer` capability
  (`internal/dataentry/nextaction.go:96-145`). Same shape, one surface over.
- **`predicatefns.AndFilters`** (`internal/predicatefns/evaluator.go:199`)
  already composes `(c1) and (c2)` from legacy filter clauses via `FromFilter`
  (`fromfilter.go:40`). ANDing `filters:` with `condition:` is existing
  machinery, not new code.
- **`GraphQuery.Any []GraphBranch`** is already a store-level DISJUNCTION,
  rendered by `buildAnySQL` (pgstore) with a matching naive implementation.
  Its doc states the property we need: "the answer is exact at the store and
  paging, counts and search stay honest with no post-filter." This is the
  precedent that makes disjunctive pushdown a bounded extension rather than a
  rewrite.
- **Identity plumbing** exists: `resolvePrincipalEntity`
  (`internal/dataentry/router.go:420`, `:474`),
  `predicatefns.WithQueryIdentity` / `ResolveQueryIdentity`,
  and the agreement check at `internal/appbuild/nextaction_matchers.go:85-92`.

**Prior art reviewed:** BUG-F1LTV0 (validator accepted an unevaluable
operator), BUG-F1LTP1 (silent operator degradation at every layer),
BUG-AMK38R (list-property flattening), BUG-WHEREWIDE (unparseable `where:`
silently widened), BUG-5OAQUG (board dropped page 2+). Four of the five are the
same failure class: a filter that silently does the wrong thing instead of
failing. That class is the primary risk this ticket must not add to.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Alternatives considered and rejected:**

| Alternative | Why rejected |
|---|---|
| Widen `filters:` / `operator:` to accept expressions | The two syntaxes overlap without erroring, so `filter.Parse` reads an expression as a property NAME and matches nothing, silently. `CLAUDE.md` and `docs/data-entry.md:1839` forbid dialect sniffing: the key IS the declaration of intent. |
| Add boolean composition to `internal/filter` | Re-opens `RES-6PK0S3`, which deliberately kept two evaluators (filter = data-shaping, predicate = conditions). The condition engine already has `and`/`or`/`not`. |
| A TypeScript predicate evaluator for boards | The fifth filter evaluator; evaluator drift is the documented root cause of BUG-F1LTV0 / BUG-F1LTP1. |
| Per-column kanban filters | A second filtering axis, and `onDrop` has no answer for "what do I write" on a predicate-defined column. A board-level condition covers the case. |
| Reuse `listPage` for the board | Its only addition over `scopedSortedEntities` is the pushdown attempt and page slice; a board wants the complete ordered set, which `scopedSortedEntities` already returns. |
| Append condition branches into `GraphQuery.Any` | Silently widens the ACL (blocker 3). |

**Technical Approach:**

### A. The kanban server path — copy `ganttHandler` (settled)

The precedent exists and is the only one: `GET /api/v1/_gantts/{id}` takes a
view CONFIG ID and applies that config's `where:` server-side
(`internal/dataentry/gantt_handler.go:89-95`, `:366-390`). Lists and kanbans
both translate config→params in the SPA instead; `/api/v1/_views/` is NOT a
view-config endpoint despite the name (it serves one entity's detail view,
`views_handler.go:471-478`).

Two of the gantt's properties are exactly what this ticket needs, both
deliberate:

- filters run **post-redaction** — "membership must not reflect a predicate
  over a value the principal cannot read" (`gantt_handler.go:345-348`)
- a match **error EXCLUDES the row and is logged** (`:376-384`) — a third
  precedent for the unset-property question, alongside next-actions
  (propagate) and automations (no-match + warning)

Shape:

- new `internal/dataentry/kanban_handler.go` with a `kanbanHandler` struct
  (`schema`, `store`, `scoped`, `redactor`), constructed next to `app.gantt`
  (`app.go:1091-1096`) with `scoped: app.scopedSortedEntities`
- registered `mux.HandleFunc("/api/v1/_kanbans/", a.kanban.handleV1Kanban)` at
  `api_v1.go:159`, plus the route probe `router_walk_test.go` requires
  (`api_v1.go:127-131`)
- tests modelled on `internal/dataentry/gantt_handler_test.go` — there are no
  Go kanban serving tests today because there is no serving code

**This is forced, not stylistic:** `app.go:184` pins
`//plimsoll:max-methods=88` and `App` has exactly 88 methods — zero headroom.
`ganttHandler`'s own doc says it is a struct because "App is at its plimsoll
method load line". A handler struct adds a FIELD (not pinned) and costs zero
methods.

`scopedSortedEntities` (`api_v1.go:368-471`) returns the COMPLETE ordered
ACL-scoped set pre-pagination — what a board needs, already consumed that way
by the gantt (empty query map, `gantt_handler.go:334`) and `/_position`.
`listPage` is NOT reusable: its only addition is the pushdown attempt and the
page slice.

### B. Where the condition evaluates

For lists, in the `applyV1Filters`/`applyRelationFilters` band of
`scopedSortedEntities` (`api_v1.go:450-457`). For boards, the same call inside
`kanbanHandler`. One Go matcher, two callers — which is the point.

Compose via `predicatefns.AndFilters` + `FromFilter`: `filters:` transpile to
predicate source and AND with `condition:`, so both keys go through ONE
evaluation rather than two passes. Empty condition yields `"true"` already.

### C. Disjunctive pushdown — smaller than feared, blocked on three decisions

The mechanical part is genuinely small. `GraphQuery.Any` is already a
disjunction with exactly **three** evaluation sites:
`buildAnySQL` (pgstore, shared with visible search),
and `collectBranchPrimes` + `matchesAny` (naive, used by fs/mem/sqlite).
Adding `Props []PropPredicate` to `GraphBranch` is ~4 lines in `buildAnySQL`
reusing the existing `propCond`, and one `matchesProps` guard in `matchesAny`.

The hard part is soundness. Three findings, all recorded on the ticket with
evidence. **Reviewed with Jeroen:** 1 is ordinary work; 2 is a real defect but
located in the LOWERING, not in `propmatch`; 3 is an authorization-ceiling
invariant that should be made structural, with better naming than
`PropPredicate`.

1. **OR is all-or-nothing.** An `or` is pushable only if EVERY arm is —
   pushing one arm drops rows the other accepts. With today's equality-only
   `PropOp` that means the atlas rule pushes nothing, since one arm is an
   ordered comparison. **That is a missing enum member, not an architectural
   limit** — dates are already `StringShaped`/byte-orderable
   (`queryplan.go:103-106`), `OrderBy` already sorts them in SQL, and
   `days_between(today(), prop) <= 2` folds to a constant bound exactly as
   `current_user.id` already does. Adding `>=`/`<=` (open question 4) makes
   the motivating rule fully pushable. Until then the board's performance
   rests on the kanban read path (A), and plain-equality disjunctions are the
   pushable case.
2. **`PropOp` lacks an operator matching Lua `~=`.** Verified: predicate `~=`
   is TRUE for an unset property, `store.PropNotEqual` EXCLUDES it. Predicate
   is right for its contract — `predicate/doc.go` commits to Lua-flavored
   equality with `nil == anything -> false`, so `nil ~= 'x'` must be true.
   `propmatch` is right for ITS contract (a filter names a population;
   `internal/filter` delegates to the same rule). **Do not change
   `propmatch`** — it would alter `filters:`, CalDAV, feeds and the CLI to fix
   a problem none of them has. Fix the LOWERING: `~=` becomes
   `(PropNotEqual v) OR (PropEqual "")`. Separately, the dialect divergence is
   author-facing and must be documented whether or not anything is pushed.
3. **The authorization part of a query is a CEILING; a view may only narrow
   it.** Same shape as the `CLAUDE.md` ACL ceiling rule (narrows only, "a bug
   fails toward less access"). `Any` is ACL-owned (sole constructor
   `internal/acl/readquery.go:178-190`); appending condition branches yields
   `acl_a OR acl_b OR cond_x` instead of `(acl_a OR acl_b) AND cond_x` — a
   condition arm becomes an alternative route to authorization. **Privilege
   escalation via `data-entry.yaml`**, invisible to any test that only checks
   the board renders correctly. Make it structural rather than documented, and
   name the field for whose AUTHORITY it carries (`Narrowing` /
   `CallerNarrowing`) rather than its structure — reusing `PropPredicate`
   invites appending to whichever slice is to hand. Strongest form: distinct
   types, so the wrong append does not compile.

Plus two consequences: the derived composite btree cannot serve an OR (decide
per-arm single-column indexes + a `BitmapOr` EXPLAIN test, or declare
disjunctive pushdown non-indexed and relax the doc consciously); and
`visiblesearch` would inherit branch Props for free while top-level `Props` is
not rendered there — an asymmetry to resolve deliberately.

**Keep the drift guard intact** (`queryplan.go:290-297`): the OR collector must
be a SEPARATE function, leaving `conditionEqualities` as the AND-spine core
both `ConditionPrefilters` and `ConditionIndexProperties` consult.

**Sequencing — DECIDED (Jeroen).** Five steps, in the ticket under
"Implementation sequence". The hazards are resolved FIRST, in small
store-level steps that each ship alone, so the feature step is the boring one:

1. `PropOp` gains a Lua-`~=` operator (finding 2)
2. `PropOp` gains `>=`/`<=` + `StringShaped` gate + `today()` folding
3. the authorization ceiling as distinct types (finding 3)
4. the feature: `condition:` key, load compile, kanban read path, Go eval, docs
5. full SQL evaluation on postgres: branch predicates + OR lowering

Step 5 stays IN this ticket rather than splitting out, because the goal is not
"add OR" but **"no Go-side condition evaluation on postgres"**.

Three more steps complete that goal; Jeroen ruled on each rather than
accepting them as boundaries:

6. **hash function** — rename off the algorithm name (it is a collision key,
   not a crypto guarantee), require pgcrypto, register the sqlite equivalent
   via `modernc.org/sqlite` `RegisterFunction`. Digest bytes must not change:
   stored, indexed, often `unique:`.
7. **field-to-field comparison** — accepted as a refactor, not a boundary. A
   predicate shape carrying two property references.
8. **`rrule_next`** — the best of the three: refuse it in conditions and move
   it to `computed:`, which already exists, materializes on write and is
   "stored and indexed exactly like authored properties" — so it becomes a
   plain column, pushable by construction. Verify the staleness question
   first (computed values materialize on write; a time-relative recurrence
   needs a recompute story).

Also open: `SQLPortable` as a single boolean is probably too coarse once
pgcrypto and a sqlite extension are in play — `fuzzy()` is already portable on
postgres via `similarity()`. A per-backend capability is the likely shape.

**Intermediate PRs are wanted** whenever a slice is already useful and
end-user noticeable — do not hold the arc for one landing.

### D. What must NOT happen

- no predicate evaluator in TypeScript (AC 4) — that is the fifth evaluator
- no condition riding through `planListPushdown` unexamined (open question 10):
  its header requires eligibility, not translation cleverness, and a silently
  ignored condition returns the unfiltered superset — `BUG-F1LTP1` again

**Files to modify:**

| File | Change |
|---|---|
| `internal/dataentryconfig/config.go` | `Condition string` on `List`, `Kanban`, `FeedSource`, `CalDAVCollection` |
| `internal/dataentryconfig/validate*.go` | compile each `condition:` at load; error names view + attribute |
| `internal/dataentry/kanban_handler.go` | NEW — `kanbanHandler`, modelled on `ganttHandler` |
| `internal/dataentry/api_v1.go` | register `/api/v1/_kanbans/`; condition eval in the filter band |
| `internal/dataentry/app.go` | construct `app.kanban` beside `app.gantt` |
| `internal/dataentry/listpushdown.go` | make a condition an explicit eligibility input |
| `internal/dataentry/router.go` | boundary identity stamp (`:420`, `:474`) |
| `internal/dataentry/feed_provider.go`, `caldav_backend.go` | condition on those surfaces + per-principal cache keying |
| `internal/store/graphquery.go` + pgstore + graphquerynaive + storetest | disjunctive pushdown (pending C) |
| `internal/queryplan/queryplan.go` | lowering + index inference agreement |
| `frontend/src/views/KanbanView.vue` | consume the endpoint; remove JS filtering; drag-drop (open question 9) |
| `frontend/src/views/KanbanView.pagination.test.ts` | rewrite — the 50-page/`has_more` assumptions no longer hold |
| `docs/data-entry.md`, `docs/caldav.md`, `CLAUDE.md` | as in Documentation Planning |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `condition:` text | operator-authored `data-entry.yaml` | compiled at LOAD against the surface's entity type via `predicatefns.Evaluator` | load error naming view + attribute; server does not start |
| entity property values | the store | typed by the metamodel through `ScalarTypeForProp` | unmodelled types omitted from the record, so a reference fails at COMPILE |
| `current_user` identity | request principal, stamped at the router boundary | `resolvePrincipalEntity` then `ResolveQueryIdentity` | unidentified request on a per-user surface fails CLOSED with a named error |

The condition is **operator config, not user input** — per root `CLAUDE.md`,
"The configuration is not a secret; the data is." It is never accepted from a
request: `FILTER_KEY_RE` (`frontend/src/utils/filters.ts:83`) is
identifier-only, so an expression cannot round-trip the URL grammar even by
accident. That property must be preserved, not relied on by luck — a test
should assert a `condition` query parameter is ignored.

**Security-Sensitive Operations:**

1. **ACL ordering.** The condition narrows an ALREADY-authorized read; it must
   never widen one. It runs after the ACL read scope, never as a replacement
   for it. Row-gating and field redaction stay where they are
   (`internal/visibility`).
2. **Redaction interaction — the load-bearing asymmetry.** The next-action
   pushdown doc (`internal/dataentry/nextaction.go:113-122`) records that the
   pre-filter compares RAW store values while the Go pass sees the candidate
   AFTER redaction, and that this is sound ONLY because redaction REMOVES a
   hidden property — it binds Nil, and every current-user form is false on Nil,
   so the Go pass is strictly narrower, never wider. A view condition inherits
   this exactly. **A condition that made a hidden property truthy would break
   it.** Pinned upstream by
   `TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse`; this ticket
   needs the equivalent per surface.
3. **Cross-principal cache leak.** Once results are principal-dependent, any
   per-collection cache (CalDAV ctag, rendered feeds) must key on the identity.
   Explicit test required — this is the highest-severity failure mode in the
   ticket, since it would serve one user's rows to another.
4. **Fail-closed on no identity.** `queryIdentityFor`
   (`internal/appbuild/nextaction_matchers.go:117-125`) already refuses both an
   unstamped ctx and the `principal.Unknown` placeholder. A per-user view must
   not silently degrade to "everyone" or to "rows where the property is unset".
   Note `ConditionPrefilters` already refuses to push an empty identity for
   this reason.
5. **Error messages.** A load error names the view, the attribute and the
   compile failure — all operator-authored config, so disclosive of nothing.
   Runtime errors must not echo entity property VALUES.

**Not a concern, deliberately:** concealing view names, property names or
condition text. Root `CLAUDE.md` settles this — config contents are already
disclosed, and code must not contort to hide them.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | How tested |
|---|---|
| 1 disjunctive eval | the six fixtures, run through the VIEW read path (not just the evaluator) — see table in the ticket |
| 2 ANDed with `filters:` | a view with both; assert a row excluded by either is absent, and that neither alone widens |
| 3 load error | malformed + unknown-attribute conditions; assert startup fails naming view and attribute |
| 4 server-side membership | board condition honoured with the SPA's JS filtering removed from the path |
| 5 `current_user` parity | same condition on all four surfaces; two principals, one CalDAV collection, different resources |
| 6 fail closed | unstamped ctx and `principal.Unknown`; assert named error, not empty and not everyone |
| 7 cache keying | populate cache as principal A, read as B, assert no bleed |
| 8 disjunctive pushdown | pgstore EXPLAIN proves index use; `storetest` holds every backend to the same answer |
| 9 paging/counts | >1 page under a partly-pushed condition; count from `CountMatched`, not `GraphCount` |
| 10 query budget | `storetest.Counting`: same query count at 10 and 50 rows |
| 11 no regression | existing conjunctive pushdown tests unchanged |
| 12 docs traps | the three documented traps match actual engine behaviour |

**Edge Cases:**

- **Unset property in a date function** — verified today as a hard EVAL ERROR,
  not false. See open question 8; whichever way it resolves needs a test.
- **`~=` vs `!=`** — `!=` is a parse error. Assert the load error is legible,
  since `filters:` beside it uses `!=`.
- **`days_between` argument order** — the reversed spelling silently matches
  everything (negative days). A fixture must prove the documented form is the
  one that works.
- **`entity.id` / `entity.type`** — do not compile today. Test whichever
  resolution question 5 takes.
- **Empty condition string** — must mean "no constraint", never "match
  nothing". `AndFilters` already returns `"true"` for empty; assert it.
- **Condition on a multi-type surface** (feeds/CalDAV may span types) — next
  actions refuse a condition not valid on EVERY named type; same rule here.
- **List-typed property** — `contains(...)` / `has_current_user(...)`, not
  `==`. The flattening defect of BUG-AMK38R must not reappear.
- **Redacted property** — binds Nil; assert the Go pass stays narrower than the
  pre-filter (security item 2).
- **Board where every row matches / no row matches** — empty column rendering.

**Negative Tests:**

- condition that does not parse → load error, server refuses to start
- condition referencing an unknown attribute → load error naming it
- condition valid on one of a surface's types but not another → load error
- `condition` supplied as a URL query parameter → ignored, not evaluated
- unidentified principal on a `current_user` surface → named error, fail closed
- a pushed disjunction that disagrees with the Go pass → `storetest` failure

**Integration approach:** the acceptance fixtures run end-to-end through the
view read path against a real store, not against `predicatefns` alone. That is
the gap that let BUG-MYN56J ship — the evaluator was fine; the surface never
called it.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| **A fifth filter evaluator.** Four exist (SPA kanban switch, SPA→param, `applyV1Filters`, `applyFilters`). A TypeScript predicate evaluator for boards would be the fifth, and evaluator drift is the documented root cause of BUG-F1LTV0/F1LTP1. | Give kanbans a GO read path; no condition evaluation in the SPA. AC 4 pins it. |
| **Inheriting a broken baseline.** Kanban filters are already broken (BUG-MYN56J). Building on them would bury a shipping defect. | `depends-on` BUG-MYN56J; sequence or fold in; AM-kanban-filter-operator-parity lands either way. |
| **Silent wrong answers** — the failure class in four of five related bugs. | Every failure mode fails at LOAD where possible; no silent widening, no silent narrowing. |
| **A caller-supplied condition widening the ACL ceiling** — the severe one. A condition arm appended to `Any` becomes an alternative route to authorization: privilege escalation via `data-entry.yaml`, passing any test that only checks the board renders correctly. | Make the ceiling structural (distinct types, or a separate field the store ANDs), named for authority rather than structure. AC 8b: a principal denied by ACL stays denied with a condition present. |
| **Unsound `~=` lowering** drops unset rows the Go pass keeps (verified). | Lower to `(PropNotEqual v) OR (PropEqual "")`; AC 8a pins unset/empty/populated on every backend. Do not "fix" `propmatch`. |
| **Author-facing dialect divergence** — `filters: "!="` excludes unset rows, `condition: "~="` includes them, in the same YAML block. Exists with or without pushdown. | Document prominently (AC 12); consider a load-time lint note for `~=` on an optional property. |
| **Half-pushed OR** drops rows silently. | Logic, not a design question: one test asserting a disjunction with any unpushable arm pushes nothing (AC 8). |
| **The optimisation may not serve the motivating case.** With today's equality-only `PropOp` the atlas rule pushes nothing, so disjunctive pushdown alone buys it nothing. | Adding ordered `PropOp` + constant folding (open question 4) makes it fully pushable, and the supporting facts are already in place. Decide deliberately at design review; do not treat it as impossible — an earlier draft did and was wrong. Board performance otherwise rests on the kanban read path. |
| **Paging/count regression** — BUG-5OAQUG already happened once on this board. | AC 9 pins >1 page; count via `CountMatched`. |
| **Cross-principal cache leak** (highest severity). | AC 7 explicit test. |
| **Scope size.** XL, and it absorbed another ticket. | Sequence into landable steps; see open question 3 on how far the kanban path goes in this ticket. |
| **TKT-UTJ24Z sequencing.** TKT-I1JU70 warns that adding filter surface before the operator-set reconciliation lands makes that job bigger. | Decide in design review whether to sequence after it; record the answer. |

**Effort:** xl (raised from l when TKT-ZQV9O5 was absorbed and the kanban
server path entered scope).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: docs written inline with the slice, see docs/data-entry.md "Conditions")

**Documentation Impact:**

- [x] `docs/data-entry.md` — the `condition:` key on lists, kanbans and feeds;
      the two-key rule; load-error behaviour; the three verified syntax traps;
      what pushes down and what does not. **Includes correcting a now-false
      published promise** at `:1920-1921` ("Anything under `or`/`not` … stays
      Go-side") — byte-identical copy at
      `docs-project/entities/guides/GUIDE-data-entry.md:1927` must move in
      lockstep. `not` stays Go-side; `or` changes.
- [x] `docs/caldav.md` — `condition:` on CalDAV collections, and the per-user
      collection case that motivated TKT-ZQV9O5
- [x] `CLAUDE.md` — extend the condition-engine section's surface list; record
      that `GraphQuery` disjunction now covers properties
- [x] ~~`docs/metamodel.md`~~ (N/A: the host-function set is unchanged; it should
      not; `days_between`/`date_add`/`rrule_next` have landed)
- [x] ~~`docs/cli-reference.md`~~ (N/A: deferred with open question 6 — the CLI
      `current_user`
- [x] ~~README.md~~ (N/A: no project-level change)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the three findings were worked through with Jeroen directly and are recorded on the ticket)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Worked through directly rather than via `/design-review`; all three are recorded on TKT-LPLZ1V under "Findings from planning" with evidence, and each is now closed in code: finding 1 by the all-or-nothing rule, finding 2 by `store.PropNotEqualOrEmpty`, finding 3 by `store.Narrowing` as a distinct type.
