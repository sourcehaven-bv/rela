---
id: TKT-N8XQ2R
type: ticket
title: 'Next-action sources accept a condition expression, evaluated before the candidate cap'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Goal

Make the dwell-time scenarios expressible. S1 ("proposal out 11 days, no reply
— chase it?") and S4 ("SOW has been in draft a while") are two of the eight
grounding scenarios in [[RES-09YLLL]] and neither can be configured today.

The `stalled` and `blocking` bands exist and are documented; an operator has no
way to say *what makes something stalled*.

## Why it is blocked today

A source's `query:` is search/filter syntax:

```text
src.Query -> queryCandidates -> executeQuery -> searchparser.ParseQuery -> internal/filter
```

`internal/filter` has no date arithmetic. `days_between(entity.due, today())`
in a `query:` hits exactly the trap [[TKT-8GD41J]] removed for `when:` clauses:
`filter.Parse` does not reject it, it mangles it into a filter on a property
literally named `days_between(entity.due, today())`, which matches nothing,
with no error at load and no warning at eval.

The host functions now exist (`days_between`, `date_add`, `rrule_next` in
`internal/predicatefns`, [[TKT-HQONQE]]). They are simply unreachable from a
next-action source.

## Scope

Add `condition:` to `NextActionSource`: a predicate expression evaluated over
the candidates `query:` selects. `query:` keeps selecting (filter syntax,
unchanged); `condition:` refines (predicate syntax).

Two keys rather than sniffing the dialect, for the reason [[TKT-8GD41J]] gives:
the syntaxes overlap without erroring, so a heuristic would guess, and guess
quietly.

### The engine must not learn about predicates

`internal/nextaction` deliberately depends on only `dataentryconfig`, `entity`
and `userstate` — it does not even reach `store`. Keep it that way with a
consumer-side interface (`docs/architecture/consumer-side-interfaces.md`):

```go
// ConditionMatcher reports whether a candidate satisfies a source's condition.
type ConditionMatcher interface {
    Match(ctx context.Context, e *entity.Entity) (bool, error)
}
```

The wiring site holds the metamodel, compiles each source's `condition:` **once
at config load**, and supplies the matcher — same shape as `CandidateFunc` and
`userstate.Store`. A bad expression then fails loudly at startup, matching
[[TKT-8GD41J]]'s fatal-on-unparseable choice.

Matchers are per-source, so the engine takes a lookup
(`func(sourceID) ConditionMatcher`) rather than putting a runtime type on
`dataentryconfig.NextActionSource`.

No arch-lint change: `nextaction` never imports `predicatefns`.

## The cap ordering is load-bearing

`eligibleFromSource` truncates to `DefaultCandidateCap` (20) **immediately**
after `e.candidates(...)`, before suppression:

```go
cands, err := e.candidates(ctx, src)
if len(cands) > e.cap { cands = cands[:e.cap] }
```

That order is fine for suppression, which is per-suggestion bookkeeping. It is
**wrong for a condition**, which is a selection predicate: filtering after the
cap means a condition matching only the 21st candidate silently never fires.

That is the same silent-no-op class this whole line of work exists to remove,
so the condition must be applied **before** truncation. Pin it with a test: a
source with >20 candidates where only a late one matches.

While here: `DefaultCandidateCap`'s comment says "see the package doc" for its
rationale and there is no such doc. Write it down — the cap is about to become
load-bearing for correctness, not just for cost.

## Out of scope

Pushing ordered comparison into SQL. `store.PropOp` stays equality-only (it
cannot consult the metamodel), so conditions evaluate in Go per candidate over
a bounded set — the same tradeoff `filter.Match` already makes, and acceptable
at next-action scale (one suggestion, capped candidates).

## Acceptance

- A source may declare `condition:`; S1 and S4 are expressible end to end.
- An unparseable `condition:` fails at config load, not at render.
- A condition matching only a beyond-the-cap candidate still fires (test).
- `internal/nextaction` still depends on only `dataentryconfig`, `entity`,
  `userstate`.
