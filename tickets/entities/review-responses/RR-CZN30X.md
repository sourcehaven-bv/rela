---
id: RR-CZN30X
type: review-response
title: WorldScope.ByType map-miss returns zero TypeResolution, which means the OPPOSITE of absence
finding: 'The plan says rule 1 (faceless types) is expressed by ABSENCE from ByType, meaning ''contribute the default state''. But w.ByType["unknown"] returns the zero TypeResolution = {Chain: nil, Fallback: FallbackExclude} = ''exclude everything of this type''. Absence and zero-value therefore mean opposite things through the same map-index expression, and Go''s syntax hides which one you got. This is an API-shape trap, not a documentation problem: it produces exactly one bug, in a backend, silently excluding a whole type.'
severity: significant
resolution: Fixed in PR-A (72bf2f21). store.WorldScope's map field is UNEXPORTED and reachable only through For(entityType) (TypeResolution, bool), which returns the two-valued answer; NewWorldScope copies its input so a scope cannot change underfoot. The type doc states the absence-vs-zero contrast explicitly and says 'Do not add a getter that collapses them'. Pinned by TestWorldScope_AbsenceIsNotTheZeroValue, which asserts ok=false for an absent type and documents that reading the returned zero value as a verdict would mean 'exclude' — the opposite of what absence means.
status: addressed
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

This does NOT relitigate the frozen `FallbackExclude = 0` decision, which is
right. It is about the container shape the plan proposes around it.

```go
type WorldScope struct { ByType map[string]TypeResolution }
```

`w.ByType["unknown-type"]` returns the zero `TypeResolution` — `{Chain: nil,
Fallback: FallbackExclude}` — which reads at every use site as "exclude
everything of this type." But the plan's Technical Approach states the opposite:
*"Rule 1 (faceless types) is expressed by absence from `ByType`"*, i.e.
absence means CONTRIBUTE THE DEFAULT STATE.

So absence and zero-value mean opposite things through the same expression, and
Go's map-index syntax hides which one you got. The plan books this as a
"comprehension trap, mitigate with godoc" — but no amount of godoc fixes an
expression that silently returns the wrong verdict. It will produce exactly one
bug, in a backend, silently excluding an entire type from a world.

**Resolution (PR-A, cheap):** never expose the map for direct indexing. Give
`WorldScope` the accessor that carries the two-valued answer:

```go
func (w WorldScope) For(entityType string) (TypeResolution, bool)
```

and make the backends' entry point a method that handles the absent case
internally. Keep the field unexported, constructed via `internal/worlds` through
a constructor. This removes the trap rather than documenting it, and it costs
nothing at this stage — the type does not exist yet.
