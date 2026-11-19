---
id: RR-T42JNTL
type: review-response
title: PermitsWorld read conferring edges with a second reader
finding: PermitsWorld had its own loop over role relations that read every tail while the new check read the zero tail. Two readers of the same fact can disagree and cost two queries per request.
severity: significant
resolution: Both now use one memo (heldRelationRoles) on the Request. PermitsWorld reads the same result. TestHoldsPermissionOnAnyEntity_ReadsOncePerRequest pins one read per role relation shared by both.
status: addressed
---
