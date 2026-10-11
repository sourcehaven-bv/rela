---
id: RR-IH08RP
type: review-response
title: Lease key by type and id
finding: 'Architecture design review: refs carry id plus face and rename hooks pass only an id.'
severity: significant
resolution: 'Plan: key by entity id for the whole family; no type argument; EntityFaceDeleted is a no-op.'
status: addressed
---
