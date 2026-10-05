---
id: RR-LK6BGO
type: review-response
title: readAddress reads the family twice
finding: readAddress called res.Family to reorder faces it already had and ignored the error.
severity: nit
resolution: Faces are sorted in place by FaceOrderOf, then by token, matching Resolver.sortFaces.
status: addressed
---
