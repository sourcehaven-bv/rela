---
id: TKT-4APGVF
type: ticket
title: Access reports answer rename like the runtime
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

`acl.Verb` is defined from `acl.Op` since TKT-0NODIZ, so `rename` is a verb. The
access reports (`rela acl can`, `who-can`, `map`) still leave it out, because
their grant helper answers rename wrongly.

`grantForRole` (internal/acl/access.go) and `grantsVerb`
(internal/acl/policy.go) read the `update:` list for rename. The runtime differs
in two cases:

- A faced type takes the family path and needs a `rename:` grant
(`decideFamily`, `grantsFamilyRename` in authz_write.go). The report would say
"allowed" where the runtime refuses.
- A faceless type also honours `rename:` (`GrantsVerbOnState` in
worldgrant.go). The report would say "denied" where the runtime allows.

## Scope

- Make the report helpers decide rename as the runtime does, with a
differential test against `AuthorizeWrite` over faced and faceless types.
- Remove the rename exclusion from `aclmap.Verbs` and add `rename` to the
`--verb` enums of `acl can`, `who-can` and `map`.
- Update the `acl map` example in docs/acl-overview.md.
