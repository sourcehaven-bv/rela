---
id: TKT-CB1YV7D
type: ticket
title: 'Configure: judge trigger edits against the ACL policy'
kind: enhancement
priority: medium
status: wont-fix
---

## Description

TKT-ZT2KREN lets the Configure space edit `flows.yaml` triggers. Trigger outputs
read and write without per-record ACL checks. The Configure check
(`internal/configedit/flows.go` `aclWriteReferences`) matches the names a
trigger writes against the text of `acl.yaml`. That guards the obvious
self-escalation routes, but it is not a policy check.

Gaps the review of TKT-ZT2KREN found:

- Reads: `{{new.x}}` / `{{old.x}}` placeholders, `on.when`, `on.where` and
`check.where` read the raw record. A `config:edit` holder can copy a value
hidden by `visible:` or a row `when:` into a record they can read.
- Subject type: `set` and `relate` write to `on.entity`, which is not judged.
- Wildcard and abstract grants in `acl.yaml` name no concrete type.
- Cascades: a new trigger can fire an unchanged trigger that writes ACL names.

Today the docs say to treat `config:edit` as read-all access. This ticket
replaces the name match with a check against the parsed `acl.Policy`, run where
the policy is available (the `cmd/rela-server` validator), and judges reads as
well as writes.

## Acceptance criteria

- An added or changed trigger that reads a property hidden from some role,
or fires on a type with row-conditional grants, is refused at preview.
- Writes are judged against the policy, including wildcard and abstract
grants and the subject type.
- Tests cover each route above.

## Resolution

Done inside TKT-ZT2KREN, on the decision to fix it in that PR.
`appbuild.CheckTriggerACL` replaces `aclWriteReferences` and judges trigger
edits against the parsed `acl.yaml` and the editor's own grants. The review
responses are recorded on TKT-ZT2KREN.
