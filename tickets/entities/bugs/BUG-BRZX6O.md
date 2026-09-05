---
id: BUG-BRZX6O
type: bug
title: role_relations silently drops write-verb keys so a role-conferring relation loads ungated
description: 'RoleRelationDef had no UnmarshalYAML, and yaml.v3 ignores keys it cannot map. An operator writing `role_relations: {member-of: {create: add-member}}` — reaching for per-verb permissions by analogy with relation_grants: — got a policy that loaded clean with requires_permission empty, i.e. the membership relation completely ungated. The one gate whose job is tamper-resistance failed open on a plausible typo.'
priority: medium
effort: s
why1: A verb key under a role_relations entry is dropped at parse time, so the delegate-X gate silently does not exist and the relation is writable by anyone holding the source-type verb grant.
why2: RoleRelationDef is a plain two-field struct with no UnmarshalYAML; yaml.v3's default behaviour is to ignore unmappable keys rather than error.
why3: The adjacent RelationWriteGrant DOES have a strict UnmarshalYAML rejecting unknown keys (and gives `read:` its own message). The stricter treatment landed on the block that widens access, not on the block that restricts it.
why4: Strictness was applied per-struct as each was written, driven by whichever failure the author had in mind at the time, rather than derived from a rule about which config surfaces must fail loudly.
why5: 'The project has no stated invariant that security-relevant config fails closed on unrecognized input. Absent one, ''unknown key'' is judged by how confusing it is to the operator rather than by which direction the failure goes — so the struct where a dropped key means LESS enforcement is exactly the one that got the lenient parser.'
status: done
prevention: 'Two things. (1) RoleRelationDef now has a strict UnmarshalYAML rejecting unknown keys, with the write verbs getting a message naming relation_grants: — the mistake is refused at load rather than discouraged in prose, and because it fails in appbuild policy load it blocks every entry point, not just the linter. (2) The docs stated the gate is all-verbs and why, and the guide example permission was renamed member-of:create -> delegate-membership: the old name read exactly like a per-verb key and was the most likely thing to suggest the broken config in the first place. The general lesson recorded here: when adding a config struct to acl.yaml, the default must be a strict unmarshaller, and the question to ask is which direction a dropped key fails — leniency is only acceptable where it fails closed.'
---

## Symptom

A policy reaching for per-verb permissions on a role-conferring relation —
by analogy with the `relation_grants:` block, which genuinely has them —
loads without complaint:

```yaml
# acl.yaml
role_relations:
  member-of:
    create: add-member
```

`create:` is not a field on `RoleRelationDef`, so yaml.v3 drops it.
`RequiresPermission` stays `""`, which every consumer reads as "this
relation is not gated". The membership relation is now writable by anyone
holding the source-type verb grant — the self-promotion escalation
(`alice --member-of--> admins`) that the gate exists to prevent.

The operator has config that reads as hardened and enforces nothing, and
nothing anywhere reports it: `RequiresPermission != ""` is the boolean
`aclaudit` A2 and the boot warning both key on, so from their point of
view this policy simply never configured a gate.

## Why it was plausible to write

`relation_grants:` — the top-level block — really does take
`create:`/`update:`/`delete:`, with a `permission:` shorthand covering all
three. So per-verb keys are a real part of the ACL vocabulary; they just
live on the block for relations that do *not* confer a role.

The security guide's own worked example made this worse by naming the
permission `member-of:create`. That is a flat opaque string gating all
verbs, but it reads like a per-verb key, and it invites the reader to
assume a matching `member-of:delete` exists.

## Fix

`RoleRelationDef.UnmarshalYAML` rejects unknown keys, mirroring the
existing `RelationWriteGrant` unmarshaller. The four write verbs get a
dedicated message, since an operator reaching for them has a coherent
intent that is simply spelled elsewhere:

```console
$ rela acl audit
appbuild: load acl.yaml: acl: parse /srv/rela/acl.yaml: role_relations:
"create" is not supported — `requires_permission:` gates ALL write verbs
on a role-conferring relation, by design. Per-verb permissions are
`relation_grants:`, which is refused on a type that confers a role
```

The failure is at policy load in `appbuild`, so it stops every entry
point rather than only the linter — an un-gated membership relation
should not boot.

## Why the gate stays flat

Worth recording, since the obvious "fix" is to add the per-verb form
rather than refuse it:

- The escalation the gate stops is a **create**. A policy gating only
  some verbs would read as hardened while stopping nothing.
- Removing a conferred edge is an availability attack, so `delete` is not
  safely left open either.
- `RequiresPermission != ""` is consumed as a boolean "is this gated" by
  `EffectiveMembershipRelation`, `aclaudit` A2/A6/A7 and the boot
  warning. Making it per-verb turns each into "gated for which verbs",
  and each is a site where a wrong answer fails open.

Per-verb CUD already exists for the non-role-conferring case, and
`relation_grants:` is refused on any type carrying this gate, so the two
can never disagree about one relation.
