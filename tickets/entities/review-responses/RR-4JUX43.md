---
id: RR-4JUX43
type: review-response
title: if_exists replace emitted the generic label on its delete half
finding: handleIfExists ran before the tagged ctx was derived so one operation produced two different labels on adjacent rows
severity: significant
resolution: 'Fixed by hoisting the writeCtx derivation above handleIfExists. Verified on the production path: the replace-delete now carries automation:replace-checklist-on-active (was bare automation) while the cascaded relation-deletes keep cascade:delete-entity:CL-001. Pinned by TestAudit_IfExistsReplaceAttributesDeleteToTheAutomation; TestAudit_IfExistsReplaceUsesCascadeLabel still passes.'
status: addressed
---

`processEntityCreations` derived its tagged `writeCtx` *after* calling
`handleIfExists`, whose replace path deletes the superseded entity. So one
operation produced two labels:

```
delete-relation  has-checklist  cascade:delete-entity:CL-001   <- correct
delete-entity    checklist      automation                     <- generic
create-entity    checklist      automation:replace-checklist-on-active
```

An operator filtering on the automation's name got the create and silently
missed the delete — the exact question this ticket exists to make answerable.

Fixed by hoisting the `writeCtx` derivation above `handleIfExists`. The cascaded
relation-deletes still keep `cascade:delete-entity:<id>`, which
`cascadeHost.DeleteEntity` stamps on a ctx it derives itself, so both labels are
now right. Pinned by `TestAudit_IfExistsReplaceAttributesDeleteToTheAutomation`;
the pre-existing `TestAudit_IfExistsReplaceUsesCascadeLabel` still passes.
