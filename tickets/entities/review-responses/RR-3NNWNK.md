---
id: RR-3NNWNK
type: review-response
title: Elevated admin.create_relation with a face widens a surface the plan calls 'no new privilege'
finding: The plan states adding a face to admin.create_relation 'widens what the elevated handle can express but not what it can reach — it is already ACL-bypassing by construction. No new privilege.' The wrong half is load-bearing. authorizeAndAudit short-circuits entirely under m.bypassACL (internal/entitymanager/manager.go:560-566), so an elevated CreateRelation never builds an acl.RelationSubject and the face is never checked against any grant. On the ordinary binding a bad face is caught by the ACL (GrantsVerbOnState, exact match); on the elevated one nothing between the Lua string and store.RelationData.FromFace examines the face except ParseFace's grammar check, because CreateRelation has no scope validation either (see RR-9LM7T7). Today admin.create_relation can only write the zero face — the one coordinate guaranteed meaningful for every relation type. After this change it can write an arbitrary grammar-valid face, including one no type declares and one on an identity-scoped type. 'Already bypasses ACL' is true of the VERB; the face is a COORDINATE, and the elevated handle currently cannot address any coordinate but the default.
severity: significant
resolution: Accepted; the user chose to keep the face on admin.create_relation but gated on the manager-side check. Plan scope item 4 now sequences the elevated binding explicitly behind item 3 (requireRelationFaceFor), because authorizeAndAudit returns early under bypassACL (manager.go:560-566) so no ACL subject is built and the manager check is the only validation left. AC 7 requires the identity-scope refusal to hold from the elevated binding as well. The plan's previous 'no new privilege' claim was removed and replaced with the verb-versus-coordinate distinction this finding drew.
status: addressed
---

## Finding

The plan's Security section says of `admin.create_relation`:

> Adding a face there widens what the elevated handle can express but not what
> it can reach — it is already ACL-bypassing by construction. No new privilege.

That is half right, and the wrong half carries the weight.

**`authorizeAndAudit` short-circuits entirely under `m.bypassACL`**
(`internal/entitymanager/manager.go:560-566`): it records the bypass and returns
`nil` before `AuthorizeWrite` is reached. So an elevated `CreateRelation` never
builds an `acl.RelationSubject` at all, and the face is never matched against
any grant.

On the **ordinary** binding, a bad face is caught by the ACL —
`GrantsVerbOnState` is exact-match, so `published` with only a draft grant is
denied. On the **elevated** one, nothing between the Lua string and
`store.RelationData.FromFace` (`manager.go:1926`) examines the face except
`entity.ParseFace`'s grammar check, because `CreateRelation` has no scope
validation either (RR-9LM7T7).

## What actually changes

Today `admin.create_relation` passes `entity.RelationOptions{}`
(`internal/lua/elevation.go:218`), so it can only ever write the **zero face** —
the one coordinate the metamodel guarantees is meaningful for every relation
type. After this change it can write an arbitrary grammar-valid face, including
one no type declares and one on an `identity`-scoped type.

**"Already bypasses ACL" is a statement about the verb.** The face is a
*coordinate*. The elevated handle currently cannot address any coordinate but
the default, so this is a genuine widening of reach, not merely of expression.

## Why the reach argument matters in practice

Elevation is per-action, gated on `allow_acl_bypass` plus an `ElevatedProvider`
(`internal/script/luascriptrunner.go:173-177`). The motivating use cases named
in `elevation.go:105-110` are authorship stamping via `created-by` — which are
**identity-scoped edges**, the exact case that needs no face.

An operator who granted `allow_acl_bypass: write` to a narrow stamping action
did not thereby consent to that action authoring edges on the published face of
a faced type. That is a different grant in substance, made silently.

## Attack path

An action declaring `allow_acl_bypass: write` — operator-approved for a stamping
job — contains, or is later edited to contain:

```lua
rela.bypass_acl(function(admin)
  admin.create_relation("POL-1", "cites", "FEAT-9", {face = "published"})
end)
```

No ACL check runs. No scope check runs. The edge lands on the published face of
a policy that the ISMS invariant says changes only by promoting a draft through
an audited copy. Reaching the same face through the gated `rela.create_relation`
would require an explicit `create: ["<source-type>@published"]` grant.

## Fix

Either:

**(a) Leave the face off `admin.create_relation` in this ticket** and say so in
Scope. The motivating cases are identity-scoped and do not need it. This is the
smaller change and costs nothing the ticket set out to deliver.

**(b) If it goes in, land RR-9LM7T7 first** — with `requireRelationFaceFor` in
the manager, the elevated path is still validated for scope and declaration even
though the ACL is bypassed, because the check sits below the bypass rather than
inside it.

Do **not** rest on the doc-comment reasoning at `elevation.go:105-116`: that
comment justifies *which methods exist* on the elevated handle, not what
arguments they accept.

## Recommendation

Option (a) for this ticket, with (b) as the follow-up that makes the elevated
face safe to add later. The ticket's stated goal is that a script can create a
faced entity and a faced content-scoped relation; neither needs the elevated
handle.
