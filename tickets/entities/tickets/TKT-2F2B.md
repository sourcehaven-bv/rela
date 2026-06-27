---
id: TKT-2F2B
type: ticket
title: 'affordances: migrate resolver to *acl.Declarative; has_role consults ancestor-conferred roles'
kind: refactor
priority: medium
effort: m
status: done
---

## Summary

Migrate the affordance resolver to consume `*acl.Declarative` instead of a flat
`*acl.Policy`, so write-authz and affordance verdicts share one resolver (one
member-of walk, one ancestor walk, one attribution set per request). Widen
`has_role` to consult ancestor-conferred roles (local + local-via-group +
local-via-ancestor + local-via-group-and- ancestor) per the four-corner Source
matrix that landed in PR 2.

**PR 3 of 4** in the ACL v1 split. Stacked on PR 2 (`feat/acl-subject-resolver`
/ TKT-GV50), which itself stacks on PR 1 (`feat/store-graphquery-dsl` /
TKT-ZYH3).

## Scope (in)

- `affordances.New(meta, lookup, *acl.Declarative)` — new signature;
drops the `*acl.Policy` first arg (reads policy via `declarative.Policy()`).
- `effective_roles.go` deleted; the role-walking job moves to
`acl.Request.ForEntity`.
- Resolver consults `acl.FromContext(ctx)` and reuses the upstream
Request when present (RR-JJYW dataentry hook, in this PR; middleware wiring
lands in PR 4).
- `bindingContext.entityRoles` populated from the Request's
per-entity attribution set so `has_role` sees ancestor-conferred grants, not
just direct ones (RR-JRPZ).
- `holdsLocalRole` deleted (subsumed by the new `entityRoles` path).
- AC8 (`TestFeature_AC8_WriteAffordanceParity`) rewritten with a
discriminating scenario (RR-Y6Y9) — old test passed even when the resolver was
wrong.
- New `TestFeature_HasRole_AncestorConferred` covers the widened
semantics.
- New `TestResolver_ReusesRequestFromContext` asserts zero re-walk
when ctx already carries a Request (RR-K7CT; fatal on premise failure).

## Scope (out)

- `dataentry.attachACLRequest` middleware → PR 4 (only the
call-site reuse of `acl.FromContext` lives here).
- `appbuild` Declarative wiring (proper StoreGraph) → PR 4.

## Acceptance criteria

1. `affordances.New` accepts `*acl.Declarative` (not `*acl.Policy`).
Existing call sites in dataentry compile + tests pass.
2. `effective_roles.go` removed; no callers remain.
3. `has_role` predicate returns true for ancestor-conferred roles.
*Test:* `TestFeature_HasRole_AncestorConferred` (belongs-to chain).
4. `Resolver.resolveViaDeclarative` reuses
`acl.FromContext(ctx)` when present; constructs a fresh Request otherwise.
*Test:* `TestResolver_ReusesRequestFromContext` (counts resolver invocations on
a stub).
5. AC8 parity test rewritten so it fails if the resolver disagrees
with `Declarative.AuthorizeWrite` (RR-Y6Y9 discriminating shape).
6. Full tree green; race-clean.

## Notes

- Per RR-WTLD `*acl.Declarative` parameter unifies the two paths;
`declarative.Policy()` is the read-only accessor.
- Per RR-JRPZ `has_role` now consults `entityRoles` populated by the
resolver — not the legacy `holdsLocalRole` shortcut.
- Per RR-Y6Y9 the old AC8 parity test was tautological; the
replacement uses a scenario where only a correct resolver agrees with the write
path.
