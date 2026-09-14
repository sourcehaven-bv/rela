---
id: TKT-EVR2TU
type: ticket
title: Named query scopes declared per entity type in schema.yaml, referenced by data-entry views
kind: enhancement
priority: high
effort: l
status: planning
---

Add `query_scopes:` to an entity type in schema.yaml: named, reusable boolean
predicate expressions that decide row membership. A list or kanban references
one by name (`query_scope: actief`) instead of writing an inline expression. A
`default` scope applies to SPA surfaces unless a view names another. Replaces
the per-view `condition:` shape, and removes the need for rela to hardcode
lifecycle concepts like `archived`.

## Problem

`filters:` on a list or kanban is a flat list of `{property, operator, value}`
entries, implicitly ANDed. There is no OR, no grouping, and no negation of a
compound. Any view whose membership rule is not a plain conjunction cannot be
expressed.

The motivating case: a `taken_bord` kanban with columns todo / bezig / wachten /
gereed, wanting all open work but only recently-completed tasks in `gereed`, so
the board does not accumulate an ever-growing done pile. The rule is inherently
disjunctive:

```text
status ~= gereed  OR  (status = gereed AND afgerond_op >= <2 days ago>)
```

The same limit hits lists: "overdue, or flagged urgent regardless of date",
"unassigned or assigned to me", "risks that are high impact or high likelihood".

An inline `condition:` per view would solve the expressiveness problem and
nothing else. Three costs show up immediately:

1. **Restatement.** The same rule is needed on a kanban, on a list, and
eventually from Lua. Each restates the expression, and they drift.
2. **No single compile site.** Every new surface that wants conditions grows
its own config walker and its own compile-at-load path.
3. **No vocabulary.** The rule is spelled out at each use, so neither an
operator nor the code can refer to "active tasks" as a thing.

## Approach

Declare the predicate on the TYPE, where it is a property of the domain rather
than of one screen:

```yaml
entities:
  taak:
    query_scopes:
      default:  "entity.status ~= 'gearchiveerd'"
      actief:   "entity.status ~= 'gereed'"
      recent:   "entity.status ~= 'gereed' or days_between(today(), entity.afgerond_op) <= 2"
      mijn:     "is_current_user(entity.toegewezen_aan)"
      archief:  "entity.status == 'gearchiveerd'"
```

```yaml
# data-entry.yaml
kanbans:
  taken_bord:
    entity_type: taak
    query_scope: recent
lists:
  archief:
    entity_type: taak
    query_scope: archief
  alles:
    entity_type: taak
    query_scope: all        # implicit, always resolves — withdraws the default
```

This is Laravel's named query scope, with its global-scope default, adapted to
rela's existing load-time-compile discipline.

### Why the name is `query_scopes:` and not `scopes:`

"Scope" is already three things in-tree, none of them this:

| existing | meaning |
| --- | --- |
| `scope:` on a relation type | identity vs content (face attachment), TKT-DOFYR1 |
| `ScopeDescriptor` / `useScopeNavigation.ts` / backend `scope.go` | the ordered result set you navigate prev/next within |
| `scope_grants:` in acl.yaml, `scopedSortedEntities` | ACL-narrowed reads |

A fourth meaning distinguished only by context would make `scopedSortedEntities`
unreadable. The `query_` prefix names the axis that is actually new.

### Identity: a scope MAY reference current_user

Scopes compile through `predicatefns.Evaluator.CompileWithCurrentUser` and
evaluate through `MatchesAs`, so `mijn:
"is_current_user(entity.toegewezen_aan)"` is a legal scope. This is the more
useful choice — "assigned to me" is one of the commonest membership rules — and
it costs less than it appears to, because the machinery already exists:

- `RequiresCurrentUser(prog)` discriminates per program, so ONE profile serves
both kinds. A scope that never mentions the identity evaluates exactly as a
plain one would, and is not held to `ErrNoCurrentUser` on an unauthenticated
deployment. Compile every scope with `current_user` declared; let the guard
decide at evaluation.
- `queryplan.conditionEqualities` ALREADY lowers `current_user.id` equalities
and the `is_current_user` / `has_current_user` sugar into
`store.PropPredicate`s. So an identity scope pushes down today with no new
lowering work, and derives an index. `tool` is deliberately not pushable (it is
diagnostic, never a membership input) and must stay that way.
- `MatchesAs` returns `ErrNoCurrentUser` rather than a non-match when the
identity is missing. Keep that: for a scope the honest outcomes are the right
rows or an error, never somebody else's rows.

Two consequences to hold:

- **A `default` scope that references `current_user` makes every list of that
type identity-dependent.** On a deployment with no identity that is a hard error
on every page, not an empty page. Surface it at load as a warning naming the
type and scope, since the operator cannot otherwise tell that a default is what
broke an anonymous deployment.
- **Never cache a scoped result set across principals.** The same rule the
gantt roll-up already carries.

### Not a security boundary

**Query scopes are UX. ACL decides what a principal may see.** A scope narrows
what a screen shows for presentation reasons; it never stands between a
principal and a row it would otherwise be entitled to. Two consequences that
must hold in the implementation:

- A scope must never be the only thing excluding a row from a response. The
ACL read gate runs independently and first.
- Therefore a scope reading a property that some role restricts via `visible:`
is a *configuration smell*, not a confidentiality bypass: the row was already
ACL-gated on its own terms. Worth a lint (see AC9), not a refusal.

This is the framing that resolves the open question inherited from the inline
`condition:` work — where a per-view condition over a redacted field looked like
a field-ACL bypass, because a view condition was the only thing deciding
membership.

Note this does NOT make a scope a substitute for `scope_grants:`. An operator
writing `mijn:` to mean "only my rows are visible" has written a UX filter, not
an access control; the ACL must independently say so. Worth saying once in the
docs, because the two read similarly in YAML.

### Relationship to `worlds:`

`worlds:` and query scopes are adjacent but are different operations, and
neither subsumes the other:

| | `worlds:` | `query_scopes:` |
| --- | --- | --- |
| operates on | faces (versions of one entity) | property values |
| semantics | rank and pick one per entity family | include or exclude the entity |
| cardinality | never changes the row count | reduces the row count |
| question | which variant of X do I show | is X here at all |

"Archived" is unambiguously the second — an archived task is not a variant of a
live task you would rank against it. So it cannot be expressed as a face.

`internal/worlds` is nonetheless the structural model to copy, for reasons its
package doc already states: the compiled form must be metamodel-free, but
compiling needs the metamodel, so the boundary gets its own package. `metamodel`
may not import `predicate`/`predicatefns` under arch-lint (confirmed: `go list
-deps ./internal/metamodel` yields only `storage` + `migration`), and
`predicatefns` already imports `metamodel`, so compiling inside `metamodel` is
both forbidden and a cycle.

One difference from worlds: `internal/worlds` may not depend on `acl`
(worldreader GUARD RULE 1 — resolution is principal-independent and must
complete before the ACL gate). `internal/scopes` is NOT principal-independent,
since a scope may read `current_user`. But it still must not import `acl`: the
identity arrives as a bound value via `predicatefns`, not as an authorization
decision. Declare `mayDependOn: [metamodel, predicate, predicatefns]` and keep
`acl` off the list, so a scope cannot start consulting grants.

### Deleting a feature rather than adding one

Once scopes exist, "archived" is not rela's business. The operator declares an
ordinary enum property and two scopes; rela never learns what archived means.
Without this, `archived: bool` ends up in the store, the query layer, the API,
the SPA, the search index and every export path, followed by "soft-deleted",
"superseded" and "draft". The repo is currently clean of any lifecycle concept
(verified: the only `archiv*` hits are theme-package zips and mime checks) and
this is how it stays clean.

## Scope

### In scope

- `query_scopes:` on `EntityDef`; structural validation in the metamodel loader.
- New `internal/scopes` package: compiles expressions to `*predicate.Program`,
invoked from `appbuild` at assembly. A bad expression is a startup failure.
- `query_scope:` on `dataentryconfig.List` and `.Kanban`, validated to name a
scope declared for that `entity_type`.
- The implicit `all` scope, always resolving to "no predicate".
- `default` applied to SPA read surfaces (lists, kanbans, list search).
- Pushdown of the store-safe part of a scope, with a mandatory Go-side re-check.
- Derived-index inference extended to account for the scope (AC11).
- Lua/MCP **opt-in** by naming a scope.

### Out of scope

- Inline `condition:` on a view. Name-only, deliberately: every predicate is
then declared in one place, so type-level tooling (lint, index derivation, the
AC9 overlap report) is exhaustive by construction.
- `default` applying to Lua, MCP, `analyze_*`, `rela validate`, tracer, or
export. These answer "what is true", not "what should a person see".
- Scopes on relation types.
- Parameterised scopes (`recent(days)`). Revisit once a real second case exists.
- Scope composition (`query_scope: [actief, mine]`). One name per view for now.
- Extending the pushdown lowering vocabulary (negation, ordered comparison).
Tracked separately; see AC8 for what that means for this ticket.

## Acceptance criteria

**AC1 — declaration and load.** A `query_scopes:` block compiles at startup. An
expression that does not compile, or references an undeclared property, is a
load error naming the type, the scope and the expression — never a silently
unevaluated scope.

**AC2 — structural validation in the loader.** Duplicate names, a name that is
not a valid identifier, and a `query_scope:` naming a scope the type does not
declare are all `SchemaValidationError`s, collected not first-wins.

**AC3 — unknown name fails closed.** A view naming a nonexistent scope refuses
at config load. It must never fall back to unfiltered — that is the difference
between a typo showing nothing and a typo showing archived records to everyone.
Mirrors `worlds.Compiled.Lookup` returning `ok=false` rather than substituting.

**AC4 — `default` applies to SPA surfaces.** With a `default` declared, a list
or kanban that names no scope returns only matching rows. Pinned for both the
pushdown and the Go path.

**AC5 — `all` withdraws.** `query_scope: all` resolves even when the type
declares no scope by that name, and returns the unfiltered set.

**AC6 — non-SPA surfaces do not inherit.** `list_entities` over MCP, a Lua
`store.query`, `analyze_cardinality`, `rela validate`, `trace_from` and export
all see the unfiltered graph with a `default` declared. A regression here means
a required relation to an excluded entity silently stops being checked — the
`rela validate`-reports-clean-over-unseen-data failure mode. One test per
surface, not one test in aggregate.

**AC7 — Lua/MCP opt in.** A script may name a scope explicitly and get the
filtered set.

**AC8 — pushdown is a superset, re-checked.** The store-safe conjuncts lower
into `store.GraphQuery.Props` via `queryplan.ConditionPrefilters`; everything
else is a Go-side filter. Identical results on both paths, pinned across
memstore and pgstore.

Today's lowering vocabulary is request-constant equalities — including
`current_user.id` and the identity sugar — but NOT negation and NOT ordered
comparison. So `mijn:` and `archief:` push down fully, while `actief:` (a `~=`)
and `recent:` (a `<=` on a date) are Go-side post-filters until the lowering
vocabulary is extended. That is acceptable for this ticket and must be
*correct*, not silently wrong: the prefilter is a superset and the Go re-check
is mandatory.

**AC9 — visible: overlap is reported, not refused.** A scope reading a property
some role restricts via `visible:` produces a startup WARNING naming the scope,
the property and the roles. Not a refusal: per "Not a security boundary" above,
ACL has already gated the row independently. The restricted set is the
COMPLEMENT of each role's `visible:` allowlist (closed-world), not a denylist,
and a `when:`-conditional grant counts as restricting.

**AC10 — no lifecycle concept in rela.** The archived story is expressible
end-to-end with an operator-declared enum plus two scopes, with no rela-side
knowledge of what archiving means.

**AC11 — derived indexes account for the scope.** `queryplan.listIndexSpec`
today derives a list's index from its `Sort` plus its static `Filters`. With a
`default` scope in play, every list of that type issues a query with extra
conjuncts the derivation does not see, so the derived index no longer matches
the shape actually probed — a sequential scan the operator was promised would
not happen, and the exact drift `ConditionIndexProperties`' doc warns about.
Extend the derivation to include the scope's pushable equalities, and prove it
with an EXPLAIN test showing a scoped list's page uses its generated index.

An identity scope is the sharp case: `mijn:` derives a real index, so getting
this wrong is a full scan on the single most-used list shape.

## Risks

- **Surface audit is the bulk of the risk, not the compile.** AC6 is a
per-consumer decision and the list is long. A missed consumer that inherits the
default silently narrows a correctness check. Mitigation: enumerate consumers
from the call sites of the read APIs and pin each one.
- **`default` is a behaviour change for existing projects.** Mitigated by it
being opt-in per type — absent `query_scopes:`, nothing changes.
- **Pushdown/Go divergence.** Mitigated by AC8 running both paths over the
same fixtures, the contract next-actions already uses.
- **An identity-referencing `default` breaks anonymous deployments loudly**
(`ErrNoCurrentUser` per page). Mitigated by the load-time warning; the hard
failure is deliberate and must not be softened to an empty page.
- **Index drift (AC11) is invisible until it is slow.** No test fails; a page
just scans. Mitigated by the EXPLAIN test, which is the only thing that actually
proves it.
