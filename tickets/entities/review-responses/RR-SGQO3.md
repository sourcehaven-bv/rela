---
id: RR-SGQO3
type: review-response
title: 'Build-vs-reuse: ''no sibling established'' precondition may already be expressible as a Lua validation rule'
finding: ValidationRule already supports Lua with read access via rela.get_entity()/rela.list_entities() (metamodel/types.go:80-93). The design's `when:` precondition ('no sibling established') overlaps with what a Lua validation rule can already express today. The design should note why a declarative predicate on the edge is preferred over an existing Lua validation rule (co-location with the transition, no Lua, first-class in the machine) rather than leaving the overlap unaddressed.
severity: minor
resolution: 'Build-vs-reuse documented: edge-local when: is chosen over a standalone Lua validation rule for co-location with the transition, declarativeness (no Lua), and first-class machine membership; it reuses internal/predicate rather than adding a third evaluator. Captured in the design doc and TransitionDef godoc.'
status: addressed
---

## Finding

`ValidationRule` already has a Lua path with read-only graph access
(`rela.get_entity()`, `rela.list_entities()`; `metamodel/types.go:80-93`). The
"no sibling established" precondition the design assigns to a transition `when:`
is already expressible as a Lua validation rule today.

## Why this matters

Not a blocker, but the design should justify adding a new `when:` mechanism
rather than pointing preconditions at existing Lua validation. The case for the
edge-local `when:` is real (co-located with the transition it guards;
declarative, no Lua; first-class in the machine so it shows in tooling/mermaid
export) — but it should be *stated*, so this isn't reinventing an existing
capability by omission.

## Resolution

Add a short build-vs-reuse note to the design: edge-local `when:` is chosen over
a standalone Lua validation rule for co-location and declarativeness, and it
reuses the predicate engine (not a third evaluator). Cross-references RES-6PK0S3
(evaluator convergence).
