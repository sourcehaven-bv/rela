---
id: RR-EU8GJ
type: review-response
title: 'when: predicate needs graph-backed host funcs that only exist in affordances today'
finding: internal/predicate is a pure (Program, Bindings)->Value engine with no graph access of its own; graph-dependent ops like 'no sibling established' require host functions (count_relations/has_relation) that are implemented by the affordances BindingContext (bindings.go:14-21,96-99), not the engine. Reusing predicate at the entitymanager write point requires re-provisioning an equivalent graph-backed BindingContext there. The graph is reachable in the manager, but this wiring does not exist outside affordances. 'Just reuse internal/predicate' is correct at the engine level but understates the host-binding plumbing.
severity: significant
status: open
---

## Finding

Verified: `internal/predicate` is standalone and reusable
(`predicate/doc.go:80-83`; only importer is `internal/affordances`). But the
engine has **no graph access** — graph-dependent operations are host functions
registered via `DeclareFunc`/`SetFunc` (`env.go:129`). "No sibling established"
compiles to something like `count_relations(entity, type) == 0`, and those host
funcs are implemented by the affordances `BindingContext`
(`affordances/bindings.go:14-21,96-99`, declared `env.go:51-54`), which holds
the graph and scans it lazily.

## Impact

Reusing the predicate engine at the entitymanager write point requires building
an equivalent graph-backed `BindingContext` there. The graph/store is reachable
in the manager, but this host-binding wiring does not exist today outside
affordances. So "reuse internal/predicate" is right at the engine layer but the
graph-query host funcs are affordances-specific plumbing that must be
re-provisioned (or extracted to a shared binding package).

## Resolution

Either (a) extract the graph-backed host-func bindings from affordances into a
shared package both affordances and the transition check consume, or (b) build a
minimal write-path BindingContext. Add this to the effort estimate — the `when:`
half is not free. Also decide the `when:` capability surface (which host funcs
are exposed at write time) as part of the design.
