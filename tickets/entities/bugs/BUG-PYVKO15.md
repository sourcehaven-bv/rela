---
id: BUG-PYVKO15
type: bug
title: Stored IdP roles outlive the deleted person and cannot be erased
description: An internal/idproles record (IdP subject, bound entity, org, role names) was removed only by an empty webhook or ambiguity. Deleting the person left it, and no admin path erased one subject (GH-2118).
priority: medium
why1: Nothing removed a record when its bound user entity was deleted or renamed.
why2: The role store sits outside the graph and was not subscribed to the entity manager's delete hook that comments, piles and leases use.
why3: TKT-IZL6NX8 designed the store around webhook-driven revocation and did not treat the record as personal data with its own retention.
prevention: Runtime state keyed by a person subscribes to the alias fanout delete hook; TestIDPRoles_ErasedWithThePerson pins it.
status: backlog
---

Fix: `idproles.Store.ForgetEntity`, called from the alias fanout on delete and
rename, and `rela acl forget-idp-roles <entity-id>` for an erasure request.
