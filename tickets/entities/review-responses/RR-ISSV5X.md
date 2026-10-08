---
id: RR-ISSV5X
type: review-response
title: 'Code: rename only re-triggers on.updated actions and the default face'
finding: A pending created/property job for the old id was lost; GetAddress with a bare id read one face.
severity: significant
resolution: EventEntityRenamed matches created, updated and property triggers; the hook reads every face raw via ListEntities(AllFaces). TestBackgroundAction_RenameReschedulesPendingCreate.
status: addressed
---
