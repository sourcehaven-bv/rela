---
id: RR-V7DH5BZ
type: review-response
title: Cascade host writes bypass the managed-type guard
finding: cascadeHost CreateEntity, WriteEntity and WriteRelation reach the store without authorizeAndAudit.
severity: minor
resolution: cascadeManagedDenial applies the rule in all three. Pinned in TestManagedTypeGuard.
status: addressed
---
