---
id: TKT-I1JU70
type: ticket
title: Let a multi-value filter choose AND (match all) alongside today's OR
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

A multi-enum list filter currently ORs its selected values: picking `Governance,
Privacy, Strategie` returns entities carrying **any** of them. That is the
documented meaning of `in` (`docs/data-entry.md:981` — "Comma-separated list;
matches any") and is what every matcher in the tree implements
(`filter.matchList`, `propmatch.equalsTarget`, `dataentry.propertyContains` all
use ANY-element).

This ticket adds the **intersection** case as an explicit, separate capability:
"entities carrying *all* of these values."

## Why OR stays the default

Raised after BUG-AMK38R shipped the tag picker, when the OR behavior became
visible for the first time. It is the right default and should not change:

- **AND is near-empty on real data.** `gebieden` values carry 3-5 areas each, so
"has all four selected areas" matches almost nothing. Each added chip would
shrink the result set toward zero, which reads as a broken filter.
- **OR is the convention for a chip strip** (Jira/GitHub labels, faceted
search): each chip widens the set, and the first chip immediately shows
something.
- **AND already exists across filters.** `GEBIEDEN` ORs within itself and is
then ANDed with `STATUS`, giving the standard `(a OR b) AND status=actief`
shape.

## Scope

- A distinct operator (working name `all`) — **do not redefine `in`.** `in` is
documented public API and appears in saved/bookmarked URLs; changing its meaning
would silently alter existing filters, the exact silent-semantics-shift failure
mode BUG-F1LTP1 and BUG-F1LTV0 were both about.
- Matcher support: `all` = every named value is present in the property's
element set. Empty value set = no constraint.
- A UI affordance on the tag picker to switch a control between any/all. Needs a
design decision: a per-control toggle in the filter bar vs. a `data-entry.yaml`
authoring option (`match: any|all`) vs. both. Prefer deciding this before
building the operator, since it determines whether the choice is per-user or
per-config.
- Config validation must accept the new operator in `filter_controls` and
static `filters:` — and reject it where it cannot be evaluated. See BUG-F1LTV0:
the validator once accepted an operator no layer could evaluate, and the views
silently returned empty.

## Out of scope

- Changing `in`/`eq`/`ne` semantics.
- AND across different properties (already the behavior).
- Negated intersection ("has none of"). `ne` already covers "has none of" for a
comma list; confirm before adding anything.

## Sequencing note

`internal/filter.Operator` has no `in` or `contains` today — the HTTP layer
implements them itself. **TKT-UTJ24Z** owns reconciling those two operator sets;
adding a third HTTP-only operator before that lands makes its job bigger. Prefer
doing this after, or as part of, that convergence.

## Acceptance criteria

1. A multi-value filter can express "match all" without changing what `in` means.
2. Selecting N values under `all` returns exactly the entities whose property
contains every one of them.
3. An empty value set under `all` is a no-op constraint, not "match empty".
4. The any/all choice is reachable from the UI and survives a URL round-trip.
5. Config validation accepts the operator wherever it is evaluable and rejects
it where it is not — no silent empty results (BUG-F1LTV0).
6. Existing `in` filters, including saved URLs, behave exactly as before.
