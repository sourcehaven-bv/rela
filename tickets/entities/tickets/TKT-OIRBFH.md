---
id: TKT-OIRBFH
type: ticket
title: 'Predicate language: current_user with is_current_user/has_current_user sugar, pushed into next-action queries'
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

There is no way to write "the things assigned to me". Neither condition dialect
knew the requesting principal: `internal/filter` had no token for it, and the
predicate language (`internal/predicate` + `internal/predicatefns`) declared no
`current_user` outside the ACL affordance profile. So a next-action
`condition:`, a view or feed `where:`, or a CalDAV collection could not select
per caller. The motivating case is a personal inbox ("tickets assigned to me
that are stale"); the gap is general.

## Scope decision (this ticket)

Originally framed as an `@me` token in the **filter** language. Redirected to
the **predicate** language, for two reasons:

- The predicate engine is the condition/policy dialect the codebase is
converging on (CLAUDE.md: "New condition/`when:`-style code evaluates through
`predicate`"), and it already has a typed `current_user` record in the ACL
affordance profile. Adding the same identity there gives one spelling across
`when:` clauses and query conditions.
- A predicate `Program` exposes its IR, so a request-constant equality can be
lowered to a store predicate (pushdown) without a second parser.

The operator writing the condition is not the "me" being selected, so the
identity is named `current_user`, not `@me`.

This ticket delivers the **language feature and its first query surface**.
Exposing it on `where:` (views, feeds, kanbans) and CalDAV is the next step and
is split into [[TKT-ZQV9O5]].

## What was built

### Language (`internal/predicatefns`)

- `current_user` is a **record** `{id, tool}` (`CurrentUserType`), matching the
affordance profile, so `has_role(current_user, entity, "editor")` in existing
`acl.yaml` keeps type-checking. Records cannot be compared with `==`, so
`entity.assignee == current_user.id` is the equality form.
- Two host functions, because Lua reserves `in` and lists cannot be compared:
`is_current_user(entity.assignee)` (string equality; an unset property is "not
me", never an error) and `has_current_user(entity.watchers)` (list membership).
Bound with an empty identity they never match.
- `DeclareCurrentUser` is deliberately **separate from `Declare`**: validation,
the scheduler and index derivation compile with no request in sight, and
referencing `current_user` there must stay a compile error at load.
- Identity is a `QueryIdentity{EntityID, Raw, Tool}` stamped on ctx once per
request by the wiring site (`WithQueryIdentity`), never derived per row. `ID()`
prefers the resolved user-entity id (so it compares against a property holding
an entity id) and falls back to the raw principal.
- **Fail closed**: `BindCurrentUser` returns `ErrNoCurrentUser` for an absent or
empty identity. An empty identity would match every unset property — the worst
version of getting this wrong.
- `Evaluator.CompileWithCurrentUser` / `MatchesAs` are distinct entry points
from `Compile` / `Matches`, and the profile is part of the program cache key.
`MatchesAs` binds the identity only when `RequiresCurrentUser(prog)` is true, so
one request-scoped profile serves conditions with and without an identity
clause. `Program.References`/`Functions` back that test; `inspect` refuses an
unhandled node type so a future node cannot make it fail open.

### Pushdown (`internal/predicate/prefilter.go`, `internal/queryplan`)

- `Program.ConstEqualities(PrefilterSpec)` walks the **top-level AND spine
only** and reports attribute-vs-request-constant equalities: non-empty string
literals, `current_user.id`, and the two sugar functions (listed by the caller
in `PrefilterSpec.ConstFuncs`; a list-typed argument is reported with `List:
true`). `or`, `not`, inequality, ordered and typed comparisons and the empty
literal push nothing — each restriction has a test asserting nothing is pushed.
- `queryplan.ConditionPrefilters` lowers those to `store.PropPredicate` behind
the same metamodel gate the filter pushdown uses (string on every type; list of
strings for membership). Membership lowers to a **non-scalar `PropEqual`**,
which every backend already defines as "some element equals" (the multi-select
rule pinned by storetest) — **no store contract change was needed.**
- `queryplan.ConditionIndexProperties` is the index-inference half; both call
one eligibility core so they cannot drift (test pins agreement).
`StaticIndexSpecs` now includes a next-action source's condition, so query and
condition equalities derive **one composite index**; `LoadStaticIndexSpecs` runs
`conditionlint` so `rela db reconcile` refuses exactly the configs the server
refuses. Membership derives no index (the btree over `->>` does not serve jsonb
containment; a GIN shape needs its own EXPLAIN test first).
- `current_user.tool` is never pushed: diagnostic, never a membership input.

### Surfaces

- **ACL affordance `when:`** (`internal/affordances`): the sugar functions are
declared beside the existing `current_user` record; `userRecordType` now aliases
`predicatefns.CurrentUserType` (drift guard test). An unidentified caller
(unstamped, or the `unknown` placeholder — reachable only through the `everyone`
role) binds NO identity, and a `when:` clause that reads the current user is
refused for them rather than evaluated against a placeholder.
- **Next-action `condition:`**: `conditionlint` compiles in the request-scoped
profile (refusing a free-text query, whose relevance cap would make the
condition lossy) and evaluates via `MatchesAs`. `nextaction.CandidateFunc` now
receives the source id; `dataentry` looks up the matcher and, through a
consumer-side `ConditionPrefilterer` capability, ANDs the condition's store-safe
conjuncts into the candidate query (`executeQueryPrefiltered`). The Go pass
stays authoritative (it also sees the field-REDACTED candidate, so it is
strictly narrower than the raw-value pre-filter). The composition root
(`appbuild.NextActionMatchers`) supplies both the matchers and a per-request
scope binder (`dataentry.NextActionRequestScope`) that derives the identity from
the resolved principal ONCE per request and refuses a boundary stamp that
disagrees with it; matchers and pre-filters only read what it stamped.
- A per-user condition for an unidentified caller (unstamped, or the `unknown`
placeholder — now `principal.Unknown`) makes that SOURCE contribute nothing,
with a WARN naming it: `nextaction.ErrIdentityRequired` is a per-source skip in
the engine, never a match against a placeholder and never a failure of the other
sources (code review RR-635ZA0). A request carrying two DISAGREEING identities —
a boundary stamp and a principal naming different users — is refused whole with
`next_action_identity_conflict` (`nextaction.ErrIdentityConflict`).

### Deliberately not exposed

State-machine `When:` (no request principal on the write path), computed
properties (persisted), validation (`context.Background()`), automation (stored
effects + transport-dependent principal), CLI `--filter` (principal is `$USER`,
not a graph identity — revisit with [[TKT-ZQV9O5]]).

## Acceptance

1. `entity.x == current_user.id`, `is_current_user(entity.x)` and
`has_current_user(entity.xs)` compile and evaluate in affordance `when:` and
next-action `condition:`; referencing them in a validation rule is a load error.
2. An absent/empty identity fails closed (`ErrNoCurrentUser`; affordance grant
refused; next-action source skipped, other sources unaffected); pushdown with an
empty identity pushes nothing.
3. Equality and membership conjuncts under a top-level `and` reach the store as
pre-filters; `or`/`not`/typed comparisons do not (tests per restriction).
4. Pushed scalar equalities and derived index columns agree (drift test).
5. End to end: two principals hitting one next-action source are offered
different entities (`TestNextAction_CurrentUserConditionEndToEnd`; verified
manually against a live `rela-server`, see the implementation checklist).
6. Full suite, golangci-lint, arch-lint, comment-lint, plimsoll, coverage floors
green.

## Follow-ups

- [[TKT-ZQV9O5]] — `where:` surfaces (views, feeds, kanbans, CalDAV) and CLI;
converge on a router-boundary identity stamp via `ResolveQueryIdentity`.
- Ordered pushdown (`due <= today() + 7`): `today()` folds to a constant, but
`store.PropOp` has no ordered operator. Needs `PropLessEqual`/`PropGreaterEqual`
across fs/mem/sqlite/pg + storetest + EXPLAIN test. See [[RES-05JD73]].
- GIN index shape for list membership, with its own EXPLAIN test.
- Enum-typed properties are not pushed by either pushdown (RR-NDER2B, deferred).
- Wizard-form conditions: the SPA passes an empty `current_user: {}`
(`DynamicForm.vue`); the client-side engine has no identity yet.
