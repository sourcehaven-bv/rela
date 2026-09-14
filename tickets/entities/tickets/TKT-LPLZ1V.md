---
id: TKT-LPLZ1V
type: ticket
title: 'condition: expressions on list/kanban/feed/CalDAV views — boolean composition, current_user, and disjunctive pushdown'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Problem

`filters:` on a list or kanban (`dataentryconfig.List.Filters`,
`Kanban.Filters`, both `[]FilterConfig` —
`internal/dataentryconfig/config.go:588` and `:775`) is a flat list of
`{property, operator, value}` entries, implicitly ANDed. `FilterConfig` has
exactly three string fields and no nesting:

```go
type FilterConfig struct {
	Property string `yaml:"property" json:"property"`
	Operator string `yaml:"operator" json:"operator"`
	Value    string `yaml:"value" json:"value"`
}
```

There is no OR, no grouping, and no negation of a compound condition. Any view
whose membership rule is not a plain conjunction cannot be expressed.
Separately, no `where:`/`filters:` surface can select per caller — the
motivating case from TKT-ZQV9O5, a CalDAV "My tasks" collection, is open for the
same reason.

### Motivating case (atlas)

The `taken_bord` kanban (`data-entry.yaml:3032`) has columns `todo`, `bezig`,
`wachten`, `gereed`. The wanted rule: show all open work, but in `gereed` show
only tasks completed in the last two days, so the board does not accumulate an
ever-growing done pile.

That rule is inherently disjunctive:

```text
status != gereed  OR  (status = gereed AND afgerond_op >= <2 days ago>)
```

Today this needs one static filter per property, all ANDed. A filter on
`afgerond_op` would also apply to the todo/bezig/wachten cards and hide them
(most have no completion date at all). So the board can be filtered globally or
not at all — neither is the wanted behaviour.

`KanbanColumn` is `{Value, Label, Icon}` only — there is **no per-column
filter**, and this ticket does not add one. A board-level disjunctive condition
expresses the rule without introducing a second filtering axis.

### Not kanban-specific

The same limit hits lists: "overdue, or flagged urgent regardless of date",
"unassigned or assigned to me", "risks that are high impact or high likelihood".

## The baseline is worse than "no OR" — read BUG-MYN56J first

A survey done while scoping this ticket found that **lists and kanbans do not
share a filter path at all**, and the kanban one is broken today:

- **A list's `filters:` never run as config on the server.** The SPA reads them
from `/_config`, translates each into a `filter[prop][op]=value` query param
(`frontend/src/components/lists/EntityList.vue:364-366`,
`frontend/src/utils/filters.ts:11-25`), and the server re-parses them as
untrusted URL params in `applyV1Filters`
(`internal/dataentry/api_v1.go:1940-2094`).
- **A kanban's `filters:` never reach the server at all.** There is no Go kanban
handler. The board fetches the entire entity type via `listAllEntities` with no
filter params (`KanbanView.vue:122-143`) and filters in JavaScript with a
three-case switch (`KanbanView.vue:230-241`) whose `default: return true` arm
makes six of the nine validator-accepted operators **silent no-ops**. The same
block flattens list properties with `String(...)` and never substitutes
`$today`.

That last one is **BUG-MYN56J**, filed separately. It matters here because this
ticket must not inherit a broken baseline: on a kanban there is no server-side
filtering to extend, so "add a condition to the existing path" has no meaning
until a path exists.

There are **four** existing filter evaluators (SPA kanban switch, SPA→param
translation, `applyV1Filters`, `applyFilters` at
`internal/dataentry/helpers.go:230-264`), which is the drift BUG-F1LTV0's why4
already named. **Adding a fifth — a TypeScript predicate evaluator for boards —
is the outcome to avoid.**

## Scope decision: merged with TKT-ZQV9O5

TKT-ZQV9O5 proposed the **same `condition:` key on the same surfaces**, in
predicate syntax, ANDed with `where:`, compiled at load, failing closed —
motivated by `current_user` rather than boolean composition. Shipping two
`condition:` keys with different semantics on one config surface would be the
bad outcome, so **this ticket absorbs it**. TKT-ZQV9O5 should be closed as
superseded when this is accepted.

The union scope:

- the `condition:` key, compiled at load, evaluated on the read path
- boolean composition (`or`, `not`, grouping)
- `current_user` / `is_current_user` / `has_current_user`
- surfaces: lists, kanbans, feeds, CalDAV collections
- per-principal cache keying and fail-closed on an unidentified request
- **a server read path for kanbans** (see above)
- **disjunctive pushdown into `store.GraphQuery`** (see below)

## Proposed approach

Add a **`condition:`** key beside `filters:` / `where:`, holding a predicate
expression, ANDed with whatever filter key the surface already has.

```yaml
kanbans:
  taken_bord:
    entity_type: taak
    column_property: status
    filters:
      - property: archief
        operator: "!="
        value: "ja"
    condition: >-
      entity.status ~= 'gereed'
      or (entity.afgerond_op ~= nil
          and days_between(today(), entity.afgerond_op) <= 2)
```

**This exact expression was verified against the live engine during planning**
(compile + evaluate, pinned clock 2026-09-13). Three things about it are traps,
all found by running it rather than reading it — see "Verified engine behaviour"
below:

- the inequality operator is **`~=`**, not `!=` (Lua syntax; `!=` is a parse
error) — while `filters:` uses `!=`, so the two keys spell it differently
- `days_between(a, b)` is days **from b to a**, so an age is
`days_between(today(), prop)`; the intuitive-looking `days_between(prop,
today())` is NEGATIVE for a past date and silently matches everything
- the `~= nil` guard is **required**: without it an entity whose date is unset
raises an eval error rather than evaluating false

This is **not a new mechanism** — it is the established pattern applied to more
surfaces:

- Next-action sources already take exactly this shape: `query:` selects,
`condition:` refines with a predicate expression, "the only place date
arithmetic works" (`docs/data-entry.md:1809`, `:1822`). The worked example is
`days_between(entity.sent_on, today()) >= 11` — structurally the atlas rule. It
is compiled at load by `conditionlint.CompileNextActions`
(`internal/conditionlint/nextaction.go:44`) and partly pushed down via
`ConditionPrefilterer` (`internal/dataentry/nextaction.go:123-145`). **This is
the closest existing analogue and the model to copy.**
- Automation `on.condition:` and validation `when_condition:`/`then_condition:`
take a predicate expression beside their filter-syntax keys.
- Root `CLAUDE.md`: `internal/predicate` is the condition engine,
`internal/filter` is the query-filter DSL. A disjunctive membership rule is a
*condition*.

The engine already gives `and`/`or`/`not`/parentheses for free, and
`predicatefns.AndFilters` (`internal/predicatefns/evaluator.go:199`) already
composes `(c1) and (c2)` from legacy filter clauses via `FromFilter`
(`internal/predicatefns/fromfilter.go:40`) — so ANDing `filters:` with
`condition:` is existing machinery, not new code.

### Two keys, not one — deliberately

`filters:`/`where:` stay as they are; `condition:` is a separate key. Do **not**
add dialect sniffing, and do **not** widen `operator:` to accept expressions.
This is the rule already stated for next actions and automations
(`docs/data-entry.md:1839`, `CLAUDE.md`): the two syntaxes overlap without
erroring, so `filter.Parse` reads `days_between(entity.due, today()) <= 7` as a
filter on a property *named* that, matches nothing, and goes quiet with no
diagnostic. **The key IS the declaration of intent.**

A `condition:` that does not compile is a **load error**, matching
`NewEngineFromMetamodel` and the next-action rule ("a typo fails at startup, not
at render"). Dropping a constraint widens the view, so failing the load is the
safe direction — cf. `BUG-WHEREWIDE` (an unparseable view `where:` silently
widened the collection) and `BUG-F1LTV0`.

## Verified engine behaviour (planning, run against the live engine)

The expression half of this feature **already works**. `days_between`,
`date_add` and `rrule_next` have landed (`internal/predicatefns/date.go:41`),
and `predicatefns.Evaluator` compiles and evaluates the motivating rule today.
Four behaviours were confirmed by running it, and each one is a trap an author
will hit:

**1. `entity.id` and `entity.type` do NOT compile.** They are bound at runtime
by `EntityRecord` (`internal/predicatefns/bind.go:38-39`) but never declared by
`EntityRecordType` (`env.go:85-100`), which walks `def.Properties` only:

```text
entity.status == 'x'    -> ok
entity.type == 'taak'   -> compile error: unknown attribute "type" on record
entity.id == 'T-1'      -> compile error: unknown attribute "id" on record
```

`internal/affordances` layers its own id/type pseudo-fields on top (`env.go`
doc, RR-TBG91). This ticket must either add them to `EntityRecordType` or state
plainly that a view condition cannot reference them. **Do not document them as
available without fixing this first.**

**2. The inequality operator is `~=`, not `!=`.** `!=` is a parse error (`syntax
error near "!"`). `filters:` uses `!=`; `condition:` requires `~=`. That
divergence is inherent to reusing Lua expression syntax and must be called out
in the docs, because the two keys sit adjacent in the same YAML block.

**3. `days_between(a, b)` is days from *b to a*** — positive while `a` is in the
future (`internal/predicatefns/date.go:102-108`). So an AGE is
`days_between(today(), prop)`. The intuitive-looking `days_between(prop,
today())` is NEGATIVE for a past date, and `-10 <= 2` is true, so **it silently
matches every old card** — the exact opposite of the intent, with no error.
Confirmed: with that spelling a `gereed` card completed 10 days ago still
matched.

**4. An unset date is an eval ERROR, not false.** `days_between(today(),
entity.afgerond_op)` on an entity with no `afgerond_op` raises `host function
argument type mismatch`. An explicit `entity.afgerond_op ~= nil and …` guard
short-circuits correctly; the truthiness shorthand (`entity.afgerond_op and …`)
is refused at compile ("'and' requires bool on left, got date").

Point 4 is a **design decision this ticket must take**, because the two existing
consumers already disagree:

- next actions RETURN the error — `internal/nextaction/nextaction.go:118`:
"An evaluation error is NOT treated as 'does not match': it is returned"
- automations treat an eval error as no-match plus a warning
(`internal/predicatefns/date.go:240-242`)

On a view, propagating the error means **one dateless card fails the whole board
request**. Treating it as no-match means a row silently vanishes — the
`BUG-WHEREWIDE` failure direction, but narrowing rather than widening. Neither
default is obviously right, which is why it is an open question below rather
than an assumption.

Verified-correct form, all six fixtures passing (pinned clock 2026-09-13):

| fixture | matches |
|---|---|
| open `todo`, no date | yes |
| `gereed` today | yes |
| `gereed` 1 day ago | yes |
| `gereed` 2 days ago | yes |
| `gereed` 3 days ago | no |
| `gereed` 10 days ago | no |

These become the acceptance fixtures for criterion 1.

## Pushdown: extend it, don't accept a post-filter

An earlier draft of this ticket treated "a disjunctive condition cannot be
pushed to the store" as a fixed constraint. **It is not.** It is a current scope
limit with a clear extension path, and the codebase already contains the
pattern.

What is true today:

- `queryplan.ConditionPrefilters` lowers only what
`predicate.Program.ConstEqualities` yields, and `collectConstEqualities`
(`internal/predicate/prefilter.go:122`) "walks the top-level AND spine only. It
deliberately does not descend into `or`, `not`, or any other node."
- `store.GraphQuery.Props` is `[]PropPredicate` — a flat conjunction by
construction (`internal/store/graphquery.go:27`), so OR has nowhere to go.

**But `GraphQuery` already expresses disjunction.** `GraphQuery.Any
[]GraphBranch` is a disjunction of per-branch predicates, ANDed with everything
else, and its doc records precisely the property this ticket wants:

> a branch's face set is applied to the CANDIDATE rows before world ranking, so
> the answer is exact at the store and paging, counts and search stay honest
> with no post-filter.

It is rendered by `buildAnySQL` (`internal/store/pgstore/graphquery.go`) as
OR-ed arms in one conjunct, with a matching naive implementation. `GraphBranch`
is today `{HasInbound, FaceIn}` — it simply has no `Props`.

So the disjunctive-pushdown work is well-shaped:

1. Give `GraphBranch` a `Props []PropPredicate` field (or introduce a small
predicate tree — AND/OR/NOT over leaves — if planning prefers generality over
reusing `Any`).
2. Render it in the two — and only two — evaluation sites: `pgstore`
(`buildPredicateParts`/`buildAnySQL`) and `graphquerynaive` (`matchesProps`,
`internal/store/graphquerynaive/naive.go:346-349`). fs/mem/sqlite share the
naive path. Both are small: `buildAnySQL` needs ~4 lines reusing the existing
`propCond`; `matchesAny` needs one `matchesProps(e, br.Props)` guard.
3. Extend the lowering so `ConstEqualities` (or a sibling) descends into `or`
and produces branches, keeping the existing soundness contract: pushed
predicates may only remove rows the Go pass would also reject.
4. Extend `storetest` so every backend is held to the new shape, per the
`CLAUDE.md` conformance rule. Note `Any` has NO test in the plain GraphQuery
suite today — it is pinned only under worlds (`storetest/worlds.go:576`,
`AnyBranchesGrantFacesPerRelation`), and those cases are about faces. A
disjunctive-Props block belongs in `storetest/graphquery.go` after
`Props_combine_with_relation_predicate`.

### Three findings from planning — read before designing

Reviewed with Jeroen; the framing below reflects that discussion. In short: **1
is ordinary work**, **2 is a genuine defect in the lowering (not in
`propmatch`)**, and **3 is an authorization-ceiling invariant that should be
made structural rather than documented.**

The investigation found three problems that make step 3 substantially harder
than "descend into `or`". None is fatal, all are design decisions, and **the
ticket's own worked example is affected**.

### The goal: no Go-side condition evaluation on postgres

Jeroen's stated target, and it is stronger than "add OR support": on the
postgres backend a `condition:` should be answered **entirely in SQL**, with the
Go pass reduced to a correctness backstop rather than a routine filter. That is
what makes paging, counts and `LIMIT` honest on a board of any size, and it is
why the pushdown steps are IN this ticket rather than deferred.

`predicate.Program` already tracks `sqlPortable` and `Profile` already has
`RequireSQLPortable` (`compile.go:26`, `:143`), and every host function carries
an `SQLPortable` flag (`predicatefns/predicatefns.go:66-102`). The machinery for
"can this whole program go to SQL?" exists; what is missing is the lowering.

**What blocks full pushdown today, and what each step does about it** — the
three restrictions are enumerated in `predicate/prefilter.go:48-61`:

| Restriction | Cleared by |
|---|---|
| Top-level AND only (no `or`) | step 5 (branch predicates + OR lowering) |
| Equality only (no ordered comparison) | step 2 (`>=`/`<=` + `StringShaped` gate) |
| Unset-row semantics differ for `~=` | step 1 (Lua-`~=` operator) |
| Constant RHS only (no field-to-field) | step 7 (two-property predicate shape) |
| `sha256()` unportable | step 6 (rename + pgcrypto + sqlite `RegisterFunction`) |
| `rrule_next()` unportable | step 8 (refuse in conditions; use `computed:`) |

Steps 6–8 are the end-goal tail: each removes one exception, and each is
independently shippable. They are listed after the feature because none of them
blocks it.

**The three apparent exceptions, and what Jeroen decided about each.** None is
accepted as permanent; each has a route, and the routes differ in kind:

1. **`sha256()` — solvable, and the "non-portable" note is too pessimistic.**
Its purpose is a **content-hash key for collision/dedup, not a cryptographic
guarantee** — `sha256Hex`'s own doc ties it to Icinga DB's content-hash key and
to `unique:` stored values (`predicatefns.go:220-234`). So: **rename to
something generic** (`content_hash` / `digest`) rather than a named algorithm,
since the name is what makes it look like a crypto primitive rather than a
keying one. Portability is an install requirement, not a barrier — **pgcrypto
may be required** (present on RDS and most hosted postgres), and sqlite is
extensible via `modernc.org/sqlite`'s `RegisterFunction`
(https://pkg.go.dev/modernc.org/sqlite#RegisterFunction). *Caveat to carry:* the
digest encoding is effectively permanent — it is a STORED, INDEXED, often
`unique:` value, so changing it later is a data migration. A rename must
therefore keep the same bytes, and the SQL spelling must produce lowercase hex
identical to the Go one, pinned by a cross-implementation test.

2. **`rrule_next()` — blocked in conditions, moved to a computed property.**
The strongest of the three, because it reframes a pushdown problem as a
modelling one. Recurrence stepping is Go logic (`metamodel.NextRrule`), not an
expression, and no SQL spelling is sensible. But `computed:` ALREADY exists and
is exactly the right home: "a pure Lua-compatible scalar expression evaluated
from the entity's other properties on every write"
(`metamodel/types.go:713-717`), materialized and "stored and indexed exactly
like authored properties" (TKT-1EM4KL). A computed `next_occurrence` is then **a
plain stored column — pushable by construction**, with no special casing, and it
gets `unique:`/index support free. Schema drift and `rela migrate gen` already
handle recomputation. So: **refuse `rrule_next` in a `condition:` at LOAD
time**, with an error naming the computed-property alternative. Note the
consequence to design around: computed values materialize on WRITE, so a
date-relative recurrence would go stale without a recompute trigger — verify
before promising this shape for time-varying expressions.

3. **Field-to-field comparison (`entity.a == entity.b`) — a refactor to take
on.** Both sides are columns, so SQL can express it; what cannot is
`PropPredicate`, which pairs a property with a literal `Value string`.
`ConstEqualities` also needs a constant to bind by construction. Accepted as
real work rather than a boundary — it needs a predicate shape carrying two
property references, in both the SQL and naive evaluators.

`match()`/`regex()`/`fuzzy()`/`contains()`/`len()` are currently unflagged.
`fuzzy()` is portable **on postgres specifically** — `similarity()` is already
used by the search backend (`pgstore/search.go:202`) — so a single boolean
`SQLPortable` is likely too coarse once pgcrypto and a sqlite extension function
are in play. **A per-backend capability is the probable shape**; fix it in
design review.

**Acceptance for the goal:** on postgres, a `condition:` using only portable
constructs is answered with **no Go-side row rejection** — asserted by comparing
rows returned by the store against rows surviving the Go pass and requiring
equality, which is stronger and more durable than matching an EXPLAIN string.
Anything still not pushable becomes a **LOAD error on postgres** (via
`RequireSQLPortable`) rather than a silent fallback to Go, so the exception set
cannot grow invisibly.

**Sequencing note (Jeroen):** this is the END GOAL, approached bit by bit.
Intermediate PRs are fine — and wanted — whenever a slice is already useful and
end-user noticeable, rather than holding everything for a big-bang landing.

### Findings from planning

**Finding 1 (a constraint, not a blocker) — OR is all-or-nothing.** AND is safe
to *sample*: pushing a subset of conjuncts only over-selects. OR is not: pushing
one arm of `A or B` drops rows where B holds and A does not — and those rows
never reach the Go pass to be rescued. So an `or` is pushable **only if EVERY
arm is**.

This is logic, not a codebase limitation, and it needs no design decision — only
a test asserting that a disjunction with any unpushable arm pushes NOTHING. What
IS a codebase choice is where the "pushable" line sits, and that is findings 2
and open question 4.

For the atlas rule that currently means no pushdown, because the second arm is
an ordered comparison and `PropOp` has only equality. **But that is a missing
enum member, not an architectural barrier** — see below. If ordered ops are
added, the motivating rule becomes fully pushable.

### The ordered-comparison gap is smaller than the `PropOp` doc implies

`PropOp`'s doc says ordered comparison "needs the property's declared type from
the metamodel … and the store layer does not consult the metamodel." That is a
statement about the STORE, and it is true. It is **not** a claim that the
comparison cannot be pushed, because the layer that builds the query does have
the metamodel and already uses it:

- `queryplan.StringShaped` (`queryplan.go:107-118`) is defined as exactly the
properties "whose byte order **IS** its order (string, enum, **date**,
**datetime**, custom type)" (`:103-106`), and `date`/`datetime` are in the
accepted set.
- `GraphQuery.OrderBy` **already sorts dates byte-wise in the store**
(`graphquery.go:78-85`), and `listpushdown.go:29-31` states ordering by such a
property "is byte-wise on both sides". So byte-ordering ISO-8601 dates in SQL is
shipping behaviour, not a new risk.
- `days_between(today(), prop) <= 2` is **not per-row computed** once `today()`
is fixed at request time: it is the constant bound `prop >= '2026-09-11'`.
Folding a request-constant to a literal before pushing is precisely what
`ConditionPrefilters` already does for `current_user.id`
(`queryplan.go:265-272`).

So the real gap is that **`PropOp` has no `PropGreaterEqual` / `PropLessEqual`
member** — one enum value plus one branch in `propCond` (pgstore) and
`matchesProps` (naive), the same two sites everything else touches. What it
needs alongside:

1. a metamodel gate restricting ordered ops to `StringShaped` properties
(integers must stay out — byte order is not numeric order, as
`listpushdown.go:31` already notes), enforced caller-side where the metamodel is
available;
2. an explicit decision on unset rows — the same class of question as blocker
2, and it must be answered the same way;
3. constant-folding of `today()`-relative bounds at the queryplan layer, with
the folded literal computed ONCE per request so the pushed bound and the Go pass
cannot disagree across a midnight boundary;
4. `storetest` coverage, since the naive and SQL paths implement it separately.

**This is step 2 of the implementation sequence.** It lands before the feature
and ships on its own, because a date bound is pushable everywhere, not just
here. An earlier draft of this ticket wrote it off as impossible; that was
wrong, and the reasoning is above.

Until it lands, a disjunction of plain equalities ("unassigned or assigned to
me", "high impact or high likelihood") is the pushable case, and the atlas
board's performance rests on the kanban read path.

**Blocker 2 — `PropNotEqual` and predicate `~=` DISAGREE on unset rows, in the
unsound direction.** Verified by running both:

| `status` | predicate `entity.status ~= 'gereed'` | store `PropNotEqual "gereed"` |
|---|---|---|
| `gereed` | false | false |
| `todo` | true | true |
| **unset** | **true** | **false** (excluded) |
| `""` | true | false (excluded) |

**Predicate's answer is the correct one for its own contract, and is not
negotiable.** `internal/predicate/doc.go` § "Equality semantics" commits to
"Lua-flavored equality, not Go-flavored", with the table entry `nil == anything
-> false`. `nil ~= 'gereed'` is therefore necessarily TRUE. A language that
advertises itself as a Lua expression subset cannot answer this differently
without breaking the promise that makes it predictable.

**`propmatch` is also not wrong — on its own terms.** It answers a FILTER-DSL
question, and its doc calls the asymmetry "deliberate and long-standing": "a
filter names the population it wants, and an entity with no status is not in the
'status is something other than doing' population." `internal/filter` delegates
to the same rule (`filter/match.go:34-38`), so `filters:`, CalDAV, feeds and the
CLI are all coherent with the store today.

So the split is **2-vs-1, and changing `propmatch` is the wrong fix** — it would
silently alter every one of those surfaces to fix a problem none of them has.

**The actual defect: one storage predicate is being asked to mean two things.**
`store.PropNotEqual` encodes "not equal AND present". That is right for the
filter DSL and wrong as a lowering target for Lua `~=`. Framed correctly,
`PropOp` simply **lacks an operator matching Lua `~=` semantics**.

Two consequences:

1. *Lowering (mechanical).* `~=` must lower to
`(PropNotEqual v) OR (PropEqual "")`, never to `PropNotEqual` alone. The branch
structure this ticket adds already expresses it. Pinned by a test with unset /
empty-string / populated fixtures on every backend. Read `storetest`
`Props_exclusion_does_not_widen` first — it pins the CURRENT meaning and must
keep passing.
2. *Author-facing divergence (documentation).* This exists **whether or not
anything is pushed down**, because the two dialects genuinely disagree:

   ```yaml
   filters:
     - property: status
       operator: "!="      # a task with NO status is EXCLUDED
       value: gereed
   condition: "entity.status ~= 'gereed'"   # a task with NO status is INCLUDED
   ```

Two adjacent keys, same apparent question, opposite answers on unset rows. Must
be documented prominently; consider a load-time lint note when a `condition:`
uses `~=` on an optional property.

This matters immediately: the atlas rule's first arm is a `~=`.

**Finding 3 — the authorization part of a query is a CEILING; nothing a view
supplies may raise it.** State it that way rather than as a fact about one
field: it is the same shape as the ACL ceiling rule in root `CLAUDE.md`
(`effective = user_grants ∩ (baseline ∪ scopes)` — a ceiling only ever NARROWS,
"so a bug fails toward less access"). A view `condition:` is caller-supplied
narrowing; the ACL query is the ceiling. The two compose by intersection, never
by union.

Today's shape makes violating that a one-line mistake. `GraphQuery.Any` is a
flat `[]GraphBranch` OR'd together, and its sole production constructor is
`internal/acl/readquery.go:178-190` — one branch per conferring relation type,
for the express purpose of stopping face-grant laundering between roles
(`readquery.go:83-89`: "a principal who reaches an entity through ONE relation
holds that relation's role and no other"). Appending condition branches to it
yields `acl_a OR acl_b OR cond_x OR cond_y` where the meaning must be `(acl_a OR
acl_b) AND (cond_x OR cond_y)`.

**That is privilege escalation via `data-entry.yaml`, not a display bug:** a
condition arm becomes an alternative route to authorization, so a principal
denied by every ACL branch is admitted by matching a view's display filter — and
every test that only checks "the board renders the right cards" still passes.

**Preferred fix: make the invariant structural, not documented.** Options in
increasing strength:

1. Guard the condition pushdown to fire only when `q.Any` is empty. Safe, but
silently disables the optimisation for exactly the ACL-complex principals, and
leaves the hazard live for the next caller.
2. A separate field so the store ANDs the two disjunctions — flattening them
is then not expressible.
3. Distinct TYPES for authorization-derived vs caller-supplied predicates, so
appending one to the other does not compile. Strongest; touches `internal/acl`,
`visibility/pushdown.go` and `listpushdown.go`.

**Naming matters here and `PropPredicate` is the wrong word to reuse** (Jeroen's
point, and it is the better framing): a name describing the STRUCTURE invites
appending to whichever slice is to hand, while a name describing WHOSE AUTHORITY
the predicate carries makes the mistake read wrong at the call site. Prefer
something like `Narrowing` / `CallerNarrowing` over another `*PropPredicate`
field. Option 3 is this argument taken to its conclusion.

Precedent for the care required: `listpushdown.go:120` already defensively
re-slices before appending to `Props`
(`append(append([]store.PropPredicate(nil), gq.Props...), props...)`) because
the ACL result is reused per principal — without the copy, one principal's
filters mutate another's cached query. Any new field needs the same treatment,
and `visibility/pushdown.go:122`'s comment enumerating the read-only fields
needs updating.

**Two further consequences to decide:**

- **Index derivation breaks its own promise.** The derived index is a
composite btree over `(properties->>p1, …)` partial on `type = X AND every
listed property is a string` (`pgstore/derivedschema.go:431-441`). It cannot
serve `a = 'x' OR b = 'y'`, and rows satisfying one arm need not have the other
property at all, so they fall outside the partial index entirely. Either derive
one single-column spec per arm and assert `BitmapOr` in a new EXPLAIN test, or
declare disjunctive pushdown explicitly **non-indexed** and relax
`ConditionIndexProperties`' doc consciously. The drift guard
(`queryplan.go:290-297`) survives cleanly only if the OR collector is a SEPARATE
function and `conditionEqualities` stays the AND-spine core.
- **`visiblesearch` would inherit branch Props for free** (it shares
`buildAnySQL`, `pgstore/visiblesearch.go:336-340`) while top-level `Props` is
NOT rendered there today — an asymmetry to resolve deliberately rather than
discover.

### What is not pushable TODAY (and what of it is genuinely fixed)

`PropOp` is equality-only (`{PropEqual, PropNotEqual}`,
`internal/store/graphquery.go:167-179`). Its doc explains the layering:

> ordered comparison (`due < 2026-01-01`) needs the property's declared type
> from the metamodel to avoid comparing dates lexicographically, and the store
> layer does not consult the metamodel. Typed comparison stays above the store.

Read precisely, that says the STORE cannot decide an ordered comparison on its
own. It does not say the comparison cannot be pushed — the caller has the
metamodel and already gates on it, dates are explicitly byte-orderable
(`queryplan.go:103-106`), and the store already sorts by them. See "The
ordered-comparison gap is smaller than the `PropOp` doc implies" above: this is
an additive enum member plus a caller-side type gate, sized on its merits rather
than ruled out.

**Genuinely not pushable, and not in scope to change:**

- ordered comparison on an **integer** property — byte order is not numeric
order (`listpushdown.go:31`)
- a comparison whose other side is **another entity field** — varies per row,
so there is no constant to bind (`predicate/prefilter.go:59-61`)
- anything under **`not`** — inverts the sense; De Morgan plus the unset-row
semantics of blocker 2 would have to be settled first
- a **free-text** query combined with a condition — `planListPushdown` already
declines outright when `q` is set (`listpushdown.go:57`)

Planning must therefore decide the **partial-pushdown** story: push the
disjunctive skeleton whose leaves are pushable, leave the rest to the Go pass,
and keep paging and counts correct when a branch is only partly pushed. A branch
that cannot be fully pushed widens that arm, which is sound (a pre-filter may
only over-select) but must not silently break `LIMIT`/count.

## Implementation sequence (five steps, decided with Jeroen)

Deliberately inverted: every hazard is resolved in a small store-level step
BEFORE the step that touches config and the SPA, so the feature step is the
boring one. Steps 1–3 each ship alone and are useful without the feature.

**Step 1 — `PropOp` gains a Lua-`~=` operator.** Not-equal-OR-empty, matching
predicate's documented Lua semantics (finding 2). Do NOT change `propmatch`:
`filters:`/CalDAV/feeds/CLI are coherent with today's `PropNotEqual` and must
stay so. `storetest` gains unset / empty-string / populated fixtures on every
backend; `Props_exclusion_does_not_widen` must keep passing unchanged. *Ships
alone: fixes a latent mismatch that would bite any future lowering.*

**Step 2 — `PropOp` gains `>=` / `<=`.** With a caller-side `StringShaped` gate
(dates and strings yes, integers no — byte order is not numeric order), and
`today()`-relative constant folding computed ONCE per request so the pushed
bound and the Go pass cannot straddle midnight. EXPLAIN test. *Ships alone:
enables date-bound pushdown everywhere, not just here.*

**Step 3 — the authorization ceiling, as distinct types.** Authorization-derived
and caller-supplied predicates become different types, so appending one to the
other does not compile (finding 3). Hardens two call sites that ship TODAY
(`helpers.go:573`, `listpushdown.go:120` — both correct now only because `Props`
is ANDed). Name for authority, not structure. *Ships alone: a security hardening
independent of this feature.*

**Step 4 — the feature.** `condition:` key on lists, kanbans, feeds and CalDAV
collections; compiled at load; the kanban Go read path (`kanbanHandler`,
modelled on `ganttHandler`); Go evaluation ANDed with `filters:`; docs. *Ships
the atlas board.* By this point every hazard is already closed.

**Step 5 — OR lowering.** Branch predicates + OR lowering, so a `condition:` of
portable constructs is answered entirely in SQL with no Go-side row rejection.
Steps 1 and 2 are what make the atlas rule reachable by it. Includes the index
derivation decision (per-arm single-column specs + `BitmapOr`, or an explicit
non-indexed declaration) and the `docs/data-entry.md:1920` correction.

**Step 6 — the hash function.** Rename away from an algorithm name (its job is a
collision key, not a crypto guarantee), require pgcrypto on postgres, register
the equivalent on sqlite via `modernc.org/sqlite`'s `RegisterFunction`. **The
digest bytes must not change** — it is a stored, indexed, often `unique:` value,
so a cross-implementation test pinning Go and SQL to identical lowercase hex is
the load-bearing part.

**Step 7 — field-to-field comparison.** A predicate shape carrying two property
references instead of a property and a literal, in both the SQL and naive
evaluators, plus a lowering that no longer requires a constant RHS.

**Step 8 — refuse `rrule_next` in conditions.** Load-time error naming the
`computed:` alternative, and docs showing the computed `next_occurrence` shape.
Verify the staleness question first: computed values materialize on write, so a
time-relative recurrence needs a recompute story before this is presented as the
recommended pattern.

### Intermediate PRs are wanted

Jeroen's instruction: ship a PR whenever a slice is **already useful and
end-user noticeable**, rather than holding the arc for one landing. Steps 1, 2,
3, 6 and 7 are each independently useful; step 4 is the user-visible feature;
steps 5 and 8 complete the goal. Do not bundle them for tidiness.

## Scope

**In scope**

1. `Condition string` on the list, kanban, feed and CalDAV-collection config
structs.
2. Compile at config load against the surface's entity type(s) via
`predicatefns.Evaluator.Compile` / `CompileWithCurrentUser`; a non-compiling
condition is a load error naming the view and the offending attribute. Mirror
`conditionlint.CompileNextActions`.
3. **A Go read path for kanbans**, reusing `listPage`/`scopedSortedEntities`, so
one server-side matcher serves lists and boards. Without it there is nothing to
evaluate a board condition in but a fifth evaluator in TypeScript.
4. Evaluate on the read path, ANDed with the surface's existing filter key (via
`predicatefns.AndFilters` + `FromFilter`).
5. `current_user` binding: stamp the identity once at the router boundary
(`predicatefns.ResolveQueryIdentity` + `WithQueryIdentity`), converging the
next-action path's per-request binder onto it; compare `Tool` as well as `ID()`
in the agreement check.
6. Fail closed: an unidentified request on a per-user surface is a named error,
never an empty or everyone's result.
7. Per-principal cache keying (CalDAV ctag, rendered feeds) once results are
principal-dependent, with an explicit cross-principal-leak test.
8. **Disjunctive pushdown** as described above, including `storetest` coverage.
9. Docs: `docs/data-entry.md` list/kanban/feed sections + `docs/caldav.md` — the
two-key rule, the load-error behaviour, what pushes and what does not.
**Including a correction that is now FALSE:** `docs/data-entry.md:1920-1921`
states "Anything under `or`/`not` … stays Go-side". That is a published promise
this ticket changes for `or`. It exists as a byte-identical copy in
`docs-project/entities/guides/GUIDE-data-entry.md:1927`; both must move in
lockstep. `not` remains Go-side. Also fix the stale cross-reference at
`pgstore/graphquery.go:280-283`, which cites "the backend-parity rule in
CLAUDE.md" — no such rule text exists there.

**Out of scope**

- Per-column kanban filters (`KanbanColumn` stays `{Value, Label, Icon}`). Note
a predicate-defined column would also break `onDrop` (`KanbanView.vue:567-595`),
which answers "what to write" with the column's `value` — a predicate has no
such answer.
- Converging `applyV1Filters` onto `internal/filter` (**TKT-UTJ24Z**) and
consolidating temporal parsing (**TKT-HFEKVN**).
- Exposing `condition:` as a user-editable/URL filter. Static operator config —
note `FILTER_KEY_RE` (`frontend/src/utils/filters.ts:83`) is identifier-only, so
an expression cannot round-trip the URL grammar anyway.
- Extending `internal/filter` with boolean composition — the condition engine
already has it; duplicating it would re-open `RES-6PK0S3`.
- **Ordered comparison in `PropOp` (`>=`/`<=`) and `today()`-relative constant
folding.** Deliberately deferred, NOT judged impossible — see the analysis above
and open question 4. It is what would make the atlas rule fully pushable, so if
design review wants that win in this ticket, move it in knowingly rather than
discovering it later.

## Related tickets

- **BUG-MYN56J** — kanban filters silently ignore six operators. **Sequence
first or fold in**: this ticket's kanban work either fixes it or must not paper
over it.
- **TKT-ZQV9O5** — absorbed by this ticket; close as superseded.
- **TKT-UTJ24Z** / **TKT-I1JU70** — the `internal/filter` vs HTTP operator-set
reconciliation. TKT-I1JU70's sequencing note says adding surface before
TKT-UTJ24Z lands makes its job bigger. **Read both before planning.**
- **TKT-SJWC7H**, **TKT-HQONQE** — `internal/expr` extraction; date arithmetic.
Confirm `days_between` has landed; the motivating case needs it.
- **TKT-OIRBFH** — shipped `current_user` + sugar + pushdown.
- **BUG-5OAQUG** — kanban silently dropped entities beyond page 1. The paging
regression to not repeat when the board gains a server path.

## Acceptance criteria

1. A list, kanban, feed and CalDAV collection each accept a `condition:` key; a
disjunctive condition is evaluated correctly. Pinned by the six fixtures in
"Verified engine behaviour" (open card with no date; `gereed` at 0, 1, 2, 3 and
10 days), run end-to-end through the view read path rather than only against the
evaluator.
2. `condition:` and the surface's existing filter key are ANDed; neither weakens
the other.
3. A condition that does not compile against the surface's entity type is a
**load error** naming the view and the offending attribute — not a silent empty
result and not a silent superset. Pinned by a test.
4. The kanban board's membership is decided **server-side**; no new filter
evaluator is added to the SPA. Pinned by a test that a board condition is
honoured with JavaScript filtering removed from the path.
5. `current_user` works identically on all four surfaces; two principals hitting
one CalDAV collection see different resources.
6. A zero/unknown principal on a per-user surface fails closed, with a test.
7. Any per-collection cache is keyed on the identity, with a test.
8. **A top-level disjunction whose EVERY arm is pushable is pushed to the
store**, not post-filtered — e.g. `entity.a == 'x' or entity.b == 'y'`. Verified
on pgstore via EXPLAIN and held for every backend by `storetest`. A disjunction
with any unpushable arm (a typed/computed comparison) pushes NOTHING and is
correct Go-side — asserted explicitly, since pushing a subset of OR arms is the
unsound case. 8z. **On postgres, a `condition:` built from SQL-portable
constructs is answered entirely in SQL** — no Go-side row rejection. Asserted by
comparing rows returned by the store against rows surviving the Go pass and
requiring equality (a stronger, more durable check than matching an EXPLAIN
string). The known non-portable constructs (`sha256`, `rrule_next`,
field-to-field comparison) are named in a test as the explicit exception list,
so the set cannot grow silently. 8a. **`~=` lowering does not drop unset rows.**
`entity.p ~= 'v'` is true for an unset `p` in the predicate language but false
under `store.PropNotEqual`. Whatever lowering is chosen, a row with `p` unset
must survive. Pinned by a test with unset, empty-string and populated fixtures
on every backend. 8b. **A condition disjunction never widens an ACL `Any`.** A
principal whose read query already carries `Any` branches gets the condition
ANDed with them, never flattened into the same OR-list. Pinned by a test with
both present.
9. Paging and counts are correct for a partly-pushed condition — no repeat of
`BUG-5OAQUG`, and the scoped count comes from `store.CountMatched`, never
`GraphCount`'s total. Pinned at more than one page of results.
10. A `storetest.Counting` budget test per the `CLAUDE.md` collection-read rule:
the query count is the same at 10 and 50 rows.
11. No pushdown regression for the purely conjunctive case.
12. Docs state the three author-facing traps verified in planning: `~=` vs
`!=` between the two adjacent keys, `days_between` argument order (with a worked
"age" example), and what an unset property does. Whichever way question 8
resolves, the docs say it explicitly.
13. Docs updated as listed. `just test`, `just lint`, `just arch-lint`,
`just coverage-check` pass.

## Open questions for planning

1. **`GraphBranch.Props` vs a predicate tree.** Reusing `Any` is the smaller
change and follows a proven pattern; a general AND/OR/NOT tree over leaves is
more expressive but touches every `Props` consumer. Decide explicitly — this is
the central store-side design choice.
2. **Partial pushdown semantics.** How a branch whose leaves are only partly
pushable is represented, and how paging/counts stay correct when one arm is
widened.
3. **RESOLVED IN SHAPE — the kanban server path copies `ganttHandler`.**
Planning found the precedent: `GET /api/v1/_gantts/{id}` already takes a VIEW
CONFIG ID and applies that config's `where:` server-side
(`internal/dataentry/gantt_handler.go:89-95`, `:366-390`). It is the ONLY
endpoint that does — lists and kanbans both translate config to params in the
SPA. Two of its properties are the ones this ticket needs, and both are
deliberate: filters run **post-redaction** ("membership must not reflect a
predicate over a value the principal cannot read", `gantt_handler.go:345-348`),
and a match **error EXCLUDES the row and is logged** rather than being swallowed
(`:376-384`) — which is a third data point for open question 8.

`ganttHandler` is its own struct explicitly because "App is at its plimsoll
method load line" (`gantt_handler.go:38-54`), and that is now literally true:
`app.go:184` pins `//plimsoll:max-methods=88` with exactly 88 methods and zero
headroom. So a `kanbanHandler` struct in a new
`internal/dataentry/kanban_handler.go`, constructed next to `app.gantt`
(`app.go:1091-1096`) taking `scoped: app.scopedSortedEntities` + a redactor
closure, costs App zero methods. It adds a FIELD, and `max-fields` is not pinned
on App. Register at `api_v1.go:159` beside `_gantts`, and add the route probe
`router_walk_test.go` requires (`api_v1.go:127-131`).

`scopedSortedEntities` (`api_v1.go:368-471`) returns the COMPLETE ordered
ACL-scoped set pre-pagination — exactly what a board wants, and already used
this way by the gantt (empty query map) and `/_position`. `listPage` itself is
NOT reusable: its only contribution over `scopedSortedEntities` is the pushdown
attempt and the page slice.

Still to decide: whether the board endpoint also gains sort/paging/pushdown
parity now, or only server-side condition evaluation. Note what changes either
way — see question 9.
4. **Ordered `PropOp` + constant-folded date bounds — in this ticket or its
own?** `days_between(today(), prop) <= 2` folds to the constant bound `prop >=
'<today-2>'`, which is pushable once `PropOp` gains `>=`/`<=`. The supporting
facts are all in place (dates are `StringShaped` and byte-orderable; `OrderBy`
already sorts them in SQL; `ConditionPrefilters` already folds request-constants
for `current_user.id`). What it needs: the enum member, one branch each in
`propCond` and `matchesProps`, a caller-side gate excluding integers, an
unset-row decision consistent with blocker 2, a single per-request fold so the
pushed bound and the Go pass cannot straddle midnight, and `storetest` coverage.

**This is the decision that determines whether the motivating case gets any
pushdown at all.** Without it, the atlas rule is Go-side (correct, but a scan);
with it, the rule is fully pushable. Weigh against the ticket already being XL.
5. **RESOLVED — `entity.id` / `entity.type` do not compile.** Confirmed by
running the engine (see "Verified engine behaviour"). Decide: add them to
`EntityRecordType` (matching what `affordances` already does for itself), or
document them as unavailable on a view condition. Adding them is the better
answer if a board condition should ever filter by type, which a multi-type
surface would need.

8. **An unset property in a date function: error or no-match?** Verified as a
hard eval error today. Next actions propagate it; automations downgrade it to
no-match-plus-warning. For a view, propagating means one bad row fails the whole
page; downgrading means rows vanish silently. Options: (a) require the author's
`~= nil` guard and propagate — explicit, but a foot-gun that only shows up once
real data has a gap; (b) downgrade to no-match + a logged warning, matching
automations; (c) make the load-time lint reject a date function applied to an
unguarded optional property, so the failure moves to startup where every other
condition error already lives. (c) fits this codebase's fail-at-load principle
best and is the recommendation to test in design review.
9. **CLI `--filter`.** The CLI principal is `$USER`, not a graph identity. Either
resolve via the ACL policy's `principal_property`, or leave `current_user`
undeclared on the CLI and say so.
10. **Free-text queries.** Next actions refuse a `condition:` on a free-text query
because the relevance cap runs before the condition. A view/feed `where:` is not
capped the same way; confirm before allowing the combination. Note
`planListPushdown` already declines outright when `q` is set
(`listpushdown.go:57`), so the pushed path is not in question — only the Go one.

11. **The board's optimistic drag-drop breaks under server-side membership.**
Today `onDrop` (`KanbanView.vue:567-600`) writes via `moveCard`, and
`beginOptimistic` (`:387-412`) rewrites the cached row; the client-side
`filteredEntities` recompute then drops the card immediately, because the same
predicate is re-evaluated over the same optimistic data. With the predicate on
the server the client cannot re-evaluate, so a card dragged INTO a state the
condition excludes stays visible until the settle refetch removes it —
flicker-then-vanish. Three sub-decisions:
    - the optimistic cache write targets `entityKeys.list(type)` (`:399`) while
the query is keyed `entityKeys.listParams(type, boardParams)` (`:133`); that
works only because the latter shares the former's prefix. A `_kanbans/{id}` key
falls OUTSIDE it and the optimistic write would hit nothing — and SSE
invalidation of `['entities', <type>]` (`:113-116`, `:631-633`) would silently
stop refreshing the board.
    - "your card left the board" has no UX today because it cannot happen; it
needs a deliberate answer (toast, or accept the refetch).
    - evaluating the predicate on BOTH sides would fix the flicker but means a
JS twin of the matcher — the fifth evaluator this ticket exists to avoid.
Dropping the optimistic step in favour of a plain refetch is the consistent
choice; confirm the latency is acceptable.

12. **Pushdown eligibility must learn about conditions — a fail-open shape.**
`planListPushdown` (`listpushdown.go:70-73`) `continue`s on any key not prefixed
`filter[`, so it inspects only filter params. Its header states the governing
invariant: "The pushed and the Go path must return the same rows in the same
order, and that is a matter of **eligibility**, not of translation cleverness."
A condition is categorically ineligible under today's rules (not an `eq`/`ne` on
one string-shaped property), so a condition-bearing list MUST either make the
planner decline or be pushed correctly. It must never ride through unexamined —
that returns the unfiltered superset, which is exactly `BUG-F1LTP1`'s failure. A
view `condition:` is config rather than a request param so it does not arrive in
`query` today, but the wiring must make this explicit and a test must pin it.
