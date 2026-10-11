---
id: RR-XHSE68
type: review-response
title: Clone hides validation errors behind an opaque 500
finding: 'handleV1CloneEntity sent every CreateEntity error to writeInternalError, so a unique: collision (always the case for a clone) became a 500 with check server logs.'
severity: significant
resolution: Clone maps *entitymanager.ValidationError to 422 validation_failed. TestCloneEntity_UniqueCollisionIs422 pins it.
status: addressed
---
