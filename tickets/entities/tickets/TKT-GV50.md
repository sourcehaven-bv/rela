---
id: TKT-GV50
type: ticket
title: 'acl: Subject + Source + Request + resolver (declarative role-based authz)'
kind: refactor
priority: medium
effort: l
status: done
---

## Summary

Build out the `acl` package internals — Subject sealed sum
(`EntitySubject`/`RelationSubject`), Source attribution kinds, Request with
Globals + ForEntity, store-backed `Graph`, role resolver with belongs-to +
`inherits-roles-through` traversal, single `NewDeclarative(p, g)` constructor,
`ReadQuery` composing `store.GraphQuery`, plus the `entitymanager` wiring that
populates Subject on every write path and surfaces `Subject.ID`/`Subject.FromID`
in the audit log.

**Out of scope for this PR** (lands in follow-ups):

- appbuild/dataentry wiring (PR 4)
- affordance-resolver migration to `*acl.Declarative` (PR 3)

Builds on the store.GraphQuery DSL (PR 1 / TKT-ZYH3).

This is PR 2 of 4 in the ACL v1 split. Reference branch: `feat/acl-v1-tkt-svxl`,
source ticket TKT-SVXL. ~1200 LOC.

## Acceptance criteria

- `acl.Subject` sealed sum with `EntitySubject{ID,Type}` and
`RelationSubject{Type,FromID,ToID}` constructors.
- `acl.Source` with `SourceKind` enum, `RoleAttribution`, deterministic
`PrimarySource` selection (sorted iteration).
- `acl.Request` exposes `Globals` (policy-globals) and `ForEntity(id)`
with cache-by-id semantics; `WithRequest(ctx, r)` / `FromContext(ctx)` helpers.
- `acl.Graph` interface + `StoreGraph` adapter built on `store.GetRelation`
surfaces non-NotFound errors (RR-K3OO).
- `acl.NewDeclarative(p, g) (*Declarative, error)` — single constructor;
`(*Declarative).Policy()` accessor (doc'd immutable).
- `acl.Policy.Validate()` rejects blank role/type/relation names (RR-NIGK).
- Role expansion sorts iteration over `RoleRelations` for determinism
(RR-MBK0).
- `acl.DepthCap` exported; lockstep with `graphquerynaive.DepthCap`
(RR-AROE, acl side).
- `acl.ReadQuery` composes `store.GraphQuery` for read-side enforcement
scaffolding (no consumers yet — affordance migration is PR 3).
- entitymanager populates `Subject` on every write; `Subject == nil`
panics (no silent fallback, RR-X1TE); audit log records
`Subject.ID`/`Subject.FromID` (RR-79HD).
- `RelationSubject.To*` not exposed if unused (RR-F9M9).
- Tests: `AuthorizeWrite_NilSubject_Panics`,
`AuthorizeWrite_UnstampedPrincipal_Denies`,
`Request_ForEntity_AttributionsDeterministic` (50 iterations),
`DepthCap_LockstepWithGraphquerynaive`, role_relations-whitespace test
(RR-ZB1V), denied-write attribution in entitymanager.

## Out-of-scope (defer to later PRs)

- `appbuild.WithACL` auto-detect / Declarative wiring → PR 4
- `dataentry.attachACLRequest` middleware → PR 4 (kept here only as
ctx helpers `WithRequest`/`FromContext`)
- Affordance resolver consumes `*acl.Declarative` → PR 3
- pgstore SQL-native GraphQuery → TKT-ZYH3 (already done in PR 1)
