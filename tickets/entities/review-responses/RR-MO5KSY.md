---
id: RR-MO5KSY
type: review-response
title: Cascade replace-delete lost the default automation triggered_by label
finding: '[security] The old cascade-host delete stamped triggered_by=automation when the ctx carried no label. Routing through Manager.DeleteEntity dropped that, so an unnamed automation''s ACL-free delete read as a direct user delete in the audit log.'
severity: minor
resolution: cascadeHost.DeleteEntity sets triggered_by=automation when the ctx names none. TestCascadeHostDelete_DeletesEveryFace asserts it on the entity delete records.
status: addressed
---
