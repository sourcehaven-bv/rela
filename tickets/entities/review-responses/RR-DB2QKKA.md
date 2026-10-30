---
id: RR-DB2QKKA
type: review-response
title: Warning used a different ACL test than its neighbours
finding: ACLPolicy() != nil vs ACLDeclarative() != nil used by validateConfigEditing and the router.
severity: nit
resolution: Uses svc.ACLDeclarative() != nil.
status: addressed
---
