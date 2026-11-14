---
id: RR-EGLP3OY
type: review-response
title: 'ACL note overstated: far end of a relation write is only readable'
finding: The doc block and guide said the one bit concerns the entity the caller may write. On a relation create or delete the far end is also checked and may only be readable, so the bit can show it has an edge to a hidden entity.
severity: significant
resolution: Reworded the relationrules.go doc block and the metamodel guide, and recorded the channel in the acl-security guide under What still leaks.
status: addressed
---
