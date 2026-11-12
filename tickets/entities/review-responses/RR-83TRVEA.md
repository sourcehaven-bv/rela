---
id: RR-83TRVEA
type: review-response
title: Bare-id reads for entries without a face
finding: Conflict entries carry no face; a bare entity.Ref read would trip the bareref archguard and name the wrong row on a faced type.
severity: minor
resolution: Gate with ReadableFacesMany by id and load rows at the face the gate returns; mapped types are not faced.
status: addressed
---
