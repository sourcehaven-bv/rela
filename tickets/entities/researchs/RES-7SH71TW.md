---
id: RES-7SH71TW
type: research
title: 'Smart lists: saved values for operator-declared filter controls'
summary: Users save values for a list's declared filter_controls instead of free expressions; rela derives a postgres index per control so every saved filter is indexed.
status: done
---

## Problem

Users want saved, reusable filters over a list ("smart lists", as in the Kan Do
app: Today, Upcoming, Flagged, plus custom ones). A free user-defined expression
gives postgres query shapes nobody declared, so no index can be prepared for
them and each one may scan the type. The operator should define which filters
are allowed, so rela can derive the indexes those filters need.

## Context

Survey of develop at `f3829fa2e`.

- Lists already declare user-facing filters: `filter_controls:` with a
`property` or a `relation` (plus `direction`)
(`internal/dataentryconfig/config.go:1154`). The user picks a value at runtime.
- Postgres already derives indexes from list config.
`queryplan.listIndexSpec` (`internal/queryplan/queryplan.go:562`) turns a list's
static `=` filters and its `sort:` keys into a `DerivedListIndex` spec.
`pgstore` reconciles it into a `rela_derived_list__` index
(`internal/store/pgstore/derivedschema.go`), with dry-run and `unenforced`
reporting.
- `filter_controls` do not feed that derivation. A runtime control value
becomes a predicate with no index behind it.
- The derivation accepts only string-shaped properties (`StringShaped`). Date
ranges, which Today and Upcoming need, are not indexed.
- Relation filter controls match on the target's display title, eq/ne only.
TKT-ZKPA1E moves them to the target id.
- No saved-filter or smart-list state exists. `internal/userstate` holds
per-suggestion state only.

## Options

### A. Free per-user predicate expression

The user writes any `internal/predicate` expression and saves it.

- Pro: most flexible.
- Con: unbounded query shapes. No index can be planned ahead, and an expensive
filter is one click away. It conflicts with the goal.

### B. Saved values for operator-declared controls

A smart list stores `{list, control key → value}` per user. Only controls the
list declares are accepted. Rela derives indexes from the declared controls.

- Pro: every query shape is known when config loads. It reuses
`filter_controls`, the list API's ACL gate and the derived-schema reconciler.
- Pro: the operator controls cost. A new filter is a config change with an
index plan behind it.
- Con: less expressive. A user cannot add a filter the operator did not
declare.

### C. Operator-declared smart lists only

The operator writes the smart lists in config; users only pick them.

- Pro: simplest, fully indexable.
- Con: no personal lists. This already exists as plain lists with static
`filters:`.

## Recommendation

Option B.

1. **Saved state.** A smart list is `{list, values}` per user, stored in a
per-user store outside the graph (like piles; `state.KV` or the tenant schema on
postgres). On load, rela checks the values against the list's current
`filter_controls` and drops keys the operator removed.
2. **Index derivation.** Do not build one composite index per subset of
controls: k controls give 2^k shapes. Derive one index per control: (type, face,
control property, then the list's sort keys). Postgres combines them with a
BitmapAnd. For a hot combination, the operator opts in to a composite, e.g.
`index: [status, assignee]` on the list. The reconciler, dry-run and
`unenforced` reporting stay as they are. Weigh the write cost of the extra
indexes; `rela db` reporting should show it.
3. **Operators per control.** Add `ops:` to a control: `eq`, `in`, and a date
`range` with relative bounds (`today`, `+7d`). The range operator needs the
derivation to accept date-shaped properties.
4. **Relation controls.** Depend on TKT-ZKPA1E. Matching on the target id makes
the filter a join the store can index, and a saved value survives a rename of
the target.

### Security

A saved smart list holds filter values only. Its rows come from the gated list
API, so it adds no read path. A relation value is an entity id in the user's own
state. A shared smart list would need the reader to re-run the query under their
own identity; out of scope for v1.

### Open questions

- Cross-type smart lists (one Today across tasks and tickets) do not fit
per-list controls. A later surface could declare a shared property contract,
such as `due` on several types. Out of scope for v1.
- Where the saved state lives (new store or an extension of piles).
- Whether built-in smart lists (Today, Upcoming) are operator config presets or
product defaults.
