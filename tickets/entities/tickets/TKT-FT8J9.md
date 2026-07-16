---
id: TKT-FT8J9
type: ticket
title: 'Resolved transition affordance: performable transitions for (principal, entity, field)'
kind: enhancement
priority: medium
effort: m
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Problem

`statemachine.Set` (TKT-E4LW2) is write-path-only. There is no way to ask the
question a status control (and CLI, and agent) actually needs:

> **For principal X, on entity Y, which transitions of field Z can they perform
> right now?**

That is the *resolved, authorized* answer — current value of Z (the `from`), the
declared out-edges, whether X holds each edge's **guard** (subject-aware against
Y), and whether each edge's **`when:` precondition** holds against Y's current
graph. It is exactly the input a Linear/Jira status dropdown needs: not the
theoretically-declared moves, but the buttons *this* principal can press on
*this* object.

Today none of that is reachable on the read side: edges, guards, and the
compiled `when:` programs are all unexported and only evaluated inside
`EnforceUpdate`.

## Scope

**In:**
- A resolving accessor on `statemachine.Set` that, given the current entity, a
guard oracle, and a graph lookup, returns the **performable** outgoing
transitions of a machine field — evaluating **both** guard and `when:` for the
single (principal, entity) — plus, for non-performable edges, *why* (guard not
held / precondition not met) so the UI can explain a disabled option.
- A `TransitionVerdicts(ctx, e)` method on `affordances.PolicyResolver` (beside
`FieldVerdicts`/`RelationVerdicts`) that calls the accessor with the resolver's
existing `acl.Declarative` + graph snapshot, producing a per-entity result the
data-entry serializer can surface.

**Out:**
- The SPA status *control* itself (consumes this; separate frontend ticket).
- A `transition:*` wire verb / `_actions` key change — decide in a follow-up once
the verdict shape is proven (this ticket produces the *data*, not necessarily a
new `_actions` entry).
- CLI "what can I do" and mermaid export (separate consumers).
- Any enforcement-behavior change on the write path.

## Design

### Predicate-on-reads is fine here (bounded, single-field)

The `internal/dataentry/CLAUDE.md` no-predicate-on-reads rule targets **hot,
unbounded paths** — per-row Lua while rendering a list of hundreds of entities.
Resolving the transitions of **one field on one entity** the user is viewing is
a bounded, on-demand, O(edges) evaluation. The rule's *rationale* (list-scan
perf cliff) does not reach this case, so evaluating `when:` here is consistent
with the rule's intent — no exception needed. (Call this out explicitly; it is
the crux.)

### statemachine: a resolving accessor (keeps predicate encapsulated)

```go
// TransitionVerdict is one resolved outgoing transition for a specific
// (principal, entity). Allowed is true iff the guard is held AND the
// precondition holds. When false, Reason names the gate that failed.
type TransitionVerdict struct {
    To      string
    Guard   string // permission name ("" = unguarded)
    Allowed bool
    Reason  string // "" when Allowed; else guard/precondition explanation
}

// Performable resolves the outgoing transitions from the current value of prop
// on e, evaluating each edge's guard (via the Guard oracle, subject-aware) and
// `when:` precondition (via the GraphLookup), for the principal on ctx. Sorted
// by To. Nil if prop is not a machine or e sits in a terminal state.
func (s *Set) Performable(
    ctx context.Context, e *entity.Entity, prop string,
    guard Guard, lookup GraphLookup,
) []TransitionVerdict
```

This reuses the existing `evalWhen` + `edgeFor` internals and the same `Guard`
/`GraphLookup` consumer-side interfaces the write path already uses. The
compiled `*predicate.Program` stays inside `statemachine` (not leaked). Guard
and `when:` evaluation is identical to `applyEdge`, minus the mutation — a
shared helper should back both so read and write can't drift.

### affordances: TransitionVerdicts(ctx, e)

`PolicyResolver` already holds an `acl.Declarative` + `RelationLookup` and
already resolves predicates per single entity (`FieldVerdicts`). Add
`TransitionVerdicts(ctx, e)` that, for each machine-typed field on e's type,
calls `Set.Performable` with a guard adapter over the resolver's ACL and its
graph lookup. Returns `map[field][]TransitionVerdict`. The data-entry serializer
surfaces it (wire shape TBD — likely alongside `_fields`).

### Guard resolution

Same subject-aware path as the write guard (`HoldsPermissionForEntity`), via the
same `statemachine.Guard` interface — the affordance supplies an adapter over
its `acl.Declarative`, exactly like appbuild's write-path `transitionGuard` but
for the current principal on ctx.

## Acceptance criteria

1. `Set.Performable(ctx, e, prop, guard, lookup)` returns the outgoing
transitions from e's current `prop` value, each with `{To, Guard, Allowed,
Reason}`, sorted by To.
2. `Allowed` is true iff the guard is held (subject-aware) AND the `when:`
precondition holds; when false, `Reason` distinguishes guard-denied from
precondition-failed.
3. Returns nil when `prop` is not a state machine or e is in a terminal state
(no out-edges); nil-safe on empty/nil Set.
4. Guard and `when:` evaluation is backed by the SAME helper the write path uses
(a regression test pins that a performable verdict and a successful
`EnforceUpdate` agree, and a non-performable one and a rejected `EnforceUpdate`
agree — read and write cannot drift).
5. `affordances.PolicyResolver.TransitionVerdicts(ctx, e)` returns per-machine-
field verdicts using the resolver's ACL + graph, for the ctx principal.
6. Subject-scoped guard works (a principal who holds the guard only via an
ownership relation to e gets `Allowed: true`; a global-less principal gets
`Allowed: false, Reason: guard`).
7. Bounded evaluation: the accessor scans only e's machine fields and their
out-edges (no store/list scan); documented as the reason predicate-on-read is
acceptable here.

## References

- Builds on TKT-E4LW2 (`internal/statemachine`) — reuses `evalWhen`, `edgeFor`,
the `Guard`/`GraphLookup` interfaces, `HoldsPermissionForEntity`.
- Sits beside `affordances.PolicyResolver.FieldVerdicts`/`RelationVerdicts`.
- Predicate-on-reads rule (and why it doesn't apply): `internal/dataentry/CLAUDE.md`.
- Consumers (separate tickets): SPA machine-aware status control; CLI; mermaid.
- Related deferred verb: TKT-XZEY (`transition:*`) — this produces the data a
`transition:*` affordance would carry, without committing to the wire verb yet.
