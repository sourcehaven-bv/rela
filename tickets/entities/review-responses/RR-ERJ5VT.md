---
id: RR-ERJ5VT
type: review-response
title: Partial cascade labels owned relations differently
finding: recordPartialCascade used cascade:delete-entity:<child> instead of the owner-delete trigger.
severity: minor
resolution: recordPartialCascade takes a trigger label. TestDeleteEntity_PartialCascadeLabelsOwnedRelations.
status: addressed
---
