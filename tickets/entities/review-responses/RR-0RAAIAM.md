---
id: RR-0RAAIAM
type: review-response
title: A 404 from the member list clears roles
finding: The first fix treated a 404 as "not a member" and cleared roles with a 200. Pratique answers 200 with an empty list for an unknown org, so a 404 only means a wrong base URL, proxy or route. Every webhook would then strip roles without a retry.
severity: significant
resolution: The 404 branch is gone; any non-200 is an error (502, retry). Pinned by the "route not found" case in TestIdpSyncAction_ErrorKeepsRoles; the docs table is updated.
status: addressed
---
