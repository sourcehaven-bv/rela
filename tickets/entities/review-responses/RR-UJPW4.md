---
id: RR-UJPW4
type: review-response
title: Subject-scoped guards can't use holdsPermission (Globals-only); needs computeForEntity path
finding: 'holdsPermission (acl/resolver.go:229) resolves only against Globals().Attributions; computeGlobals (resolver.go:15-48) includes only static assignments + member-of group roles + everyone, NOT relation-conferred local roles (godoc: ''global-only by design''). The subject-aware path is computeForEntity (authz_write.go:39). So the ticket''s headline ''assignee may transition their own ticket'' (role conferred by an ownership relation) will NOT work via holdsPermission - guard resolution must go through the entity-ID-aware computeForEntity path, which requires the subject ID and ties to the ordering finding. Revises the ''~15 lines, near-copy of delegate-permission gate'' estimate: delegate gate uses globals; subject-scoped guard needs local-role machinery.'
severity: significant
resolution: Guard resolves via the subject-aware acl.Request.HoldsPermissionForEntity (backed by computeForEntity), NOT the globals-only holdsPermission. Own-subject scoping via relation-conferred roles works. Test TestRequest_HoldsPermissionForEntity_SubjectScoped.
status: addressed
---

## Finding

The design's ⚠️ note is confirmed and should be promoted from caveat to a
committed decision. `holdsPermission` (`internal/acl/resolver.go:229`) resolves
only against `r.Globals(ctx).Attributions`; `computeGlobals` (resolver.go:15-48)
unions **only** static per-user assignments + `member-of` group roles + the
`everyone` role. It explicitly does **not** include relation-conferred local
roles (godoc at resolver.go:221,227-228 says permissions are "global-only by
design").

The subject-aware path is `computeForEntity` (`authz_write.go:39`, defined
resolver.go:118): it starts from `Globals()` then adds local-role probes along
edges from the principal's member-set to the target entity (and ancestors via
inheritance).

## Impact

The ticket's headline subject-scoping story — "assignee may move their own
ticket to done," conferred by a role-relation on the ownership edge — **will not
work** if the guard resolves via `holdsPermission`. The relation-conferred role
is invisible there. So the two behaviors the design promises are split across
two resolvers:

- global capability ("privacy-officer may establish anything") → `holdsPermission`
- subject-scoped ("owner may establish their own") → `computeForEntity`

## Resolution

Guard resolution must go through the **entity-ID-aware** path
(`computeForEntity` / `authorizeEntityWrite`, authz_write.go:34-44), not
`holdsPermission`. That means the guard check needs the subject's ID — which
ties directly to the ordering finding (old-load must precede the guard check,
and the check must be subject-aware). Update the "~15 lines, near-copy of the
delegate-permission gate" estimate: the delegate gate uses globals; a
subject-scoped transition guard needs the local-role machinery, so it is closer
to the entity-write authz path. Still small, but not the delegate gate.
