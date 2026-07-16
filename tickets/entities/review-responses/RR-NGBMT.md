---
id: RR-NGBMT
type: review-response
title: Inline type:enum properties silently get no machine — usability cliff, no warning
finding: 'Inline `type: enum` + values (PropertyDef, validation.go:334-358) and named CustomType (validation.go:385-389) are separate code paths never merged. Putting transitions on CustomType cleanly means named-type-only, but every existing inline `status: {type: enum}` silently gets no state-machine enforcement and nothing warns the author. Most status enums in-tree are likely inline. Migration/opt-in ergonomics and a lint (''inline enum named like a lifecycle field has no machine'') should be considered.'
severity: minor
resolution: Machines are named-CustomType-only by design (inline enums carry no transitions). Documented on CustomType.Transitions godoc. The stricter lint for lifecycle-named inline enums is deferred as a nice-to-have; the core distinction is intentional and documented.
status: addressed
---

## Finding

Confirmed: inline `type: enum` and named CustomType are distinct paths, never
normalized (`validation.go:334-358` vs `385-389`; `ResolveWidgetFromType`
branches them separately, `schema_output.go:117-139`). So `transitions:` on
CustomType means machines are **named-type-only**, exactly as the design intends
— but the consequence is a usability cliff: an author with an inline `status:
{type: enum, values: [...]}` gets **no** enforcement and **no** warning. The
existing in-tree status enums are the likely victims.

## Resolution

Acknowledge explicitly in the design and consider: (a) a migration note / helper
to promote inline enums to named types, and (b) an optional load-time lint that
flags an inline enum whose property name suggests a lifecycle (`status`,
`state`) and has no machine. Not blocking, but call it out so it's a decision,
not a silent gap.
