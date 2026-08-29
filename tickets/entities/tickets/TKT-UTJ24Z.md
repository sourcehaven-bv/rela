---
id: TKT-UTJ24Z
type: ticket
title: Converge applyV1Filters onto internal/filter — one matcher, one operator set
kind: refactor
priority: medium
effort: l
status: backlog
---

## Description

`internal/dataentry.applyV1Filters` (`api_v1.go:1757-1886`) is a second,
independent implementation of property matching that duplicates
`internal/filter.Match`. The duplication has already produced one shipped defect
(BUG-AMK38R: list-typed properties could never match, because `applyV1Filters`
flattened them with `fmt.Sprintf("%v")` while `filter.matchList` handled them
correctly).

This ticket removes the duplication. BUG-AMK38R fixes the user-visible symptom
behind a local seam; this fixes the reason the symptom was possible.

It is the sibling of **TKT-HFEKVN**, which covers the temporal-parsing half of
the same function — `helpers.go:72` already carries a `TODO(TKT-HFEKVN)` noting
three copies that "already disagree."

## Why this is not a drop-in

Three divergences must be reconciled deliberately; each would otherwise silently
change behavior for existing callers:

1. **Operator set.** `filter.Operator` has eight values and **no `in`,
no `contains`** (`OpIn`/`OpContains` appear nowhere in `internal/`). The HTTP
API supports both and `docs/data-entry.md:1081` documents them as public. Either
the core gains two operators, or the API loses documented behavior. Decide
explicitly.
2. **`eq` glob semantics.** `filter.matchString` (`match.go:207-215`) honors
`IsGlob`, so a value containing `*`/`?` becomes a pattern. HTTP `eq` is literal.
Converging silently changes the meaning of any filter value containing a glob
character.
3. **Ordered operators on strings.** `filter.validateOperatorForType`
(`match.go:157-173`) rejects `<`/`<=`/`>`/`>=` on a string property; the HTTP
path permits them and falls back to lexicographic `compareValues`. Requests that
work today would begin returning 400.

## Sequencing

**After TKT-HFEKVN.** That ticket rewrites `compareValues`'s temporal parsing
inside this same function; doing them in the wrong order means touching it twice
and resolving the same conflicts again.

## Blast radius

`filter.Match` is consumed by CalDAV (`caldav_backend.go:430`), feeds
(`feed_provider.go:251`), search/scope helpers (`helpers.go:887`) and view
sections (`views.go:221`). None are broken today, so any change in shared
semantics is a regression risk in surfaces this ticket is not otherwise
touching. That blast radius is precisely why this was split out of BUG-AMK38R
rather than bundled into it.

## Acceptance criteria

1. One matcher decides whether a property value satisfies a filter clause;
`applyV1Filters` no longer contains a parallel comparison loop.
2. The `in`/`contains` question is resolved explicitly — either promoted into
the shared operator set with tests, or documented as removed from the API with a
migration note. Not left implicit.
3. Glob-vs-literal `eq` semantics for the HTTP path are stated and pinned by a
test, whichever way they resolve.
4. Ordered-operator-on-string behavior is stated and pinned by a test.
5. The list-semantics tests added by BUG-AMK38R still pass unchanged — the
convergence must not regress the fix that motivated it.
6. Existing CalDAV, feed, view-section and CLI filter tests pass without
modification.

## Additional scope (from BUG-AMK38R code review)

Two defects were confirmed during that review, deferred here because fixing
them changes behavior for SCALAR properties on a path BUG-AMK38R did not
otherwise touch — the same blast-radius reason this ticket exists.

4. **An empty `in`/`ne` value is not the complement of a populated one.**
   `strings.Split("", ",")` yields `[""]` — a one-element set containing the
   empty string, not an empty set — and the top-of-loop missing-property branch
   (`api_v1.go`, the `!ok` case) special-cases `eq`+empty only, dropping the row
   for every other operator. So `filter[x][in]=` and `filter[x][ne]=` do not
   partition the row set. The relation pass already does the sane thing
   (`if want == "" { continue }`, i.e. no constraint); the property pass should
   agree. Verified pre-existing: identical before and after BUG-AMK38R.

5. **`in`/`ne` trim the filter side but not the property side.**
   `filter[x][in]=" leading"` does not match a property value `" leading"`,
   while `filter[x]=" leading"` does — the same value answered two ways
   depending on operator. The trim is defensible for hand-typed URLs; the
   asymmetry with `eq` is not. Decide one rule and apply it to both.

Both are recorded as deferred review-responses on BUG-AMK38R (RR-NTSTPD,
RR-D3DH50).

## Wire-format note

BUG-AMK38R made the backend read the `[]` array form correctly (each repeated
param is one member, verbatim), which is what lets a value containing a comma
round-trip. The SPA still cannot EMIT that form: `FilterState` holds one string
per key, so a multi-selection is comma-joined and a comma-bearing enum value
cannot be expressed. Mitigated there by keeping a single selection on `=` (which
compares whole and is always correct); a full fix means teaching `FilterState`
to carry multiple values, which belongs with this ticket's wire-format work.
