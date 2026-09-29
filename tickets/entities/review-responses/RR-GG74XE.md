---
id: RR-GG74XE
type: review-response
title: bypass_acl writes one acl-bypass record per face
finding: Under bypass_acl a family delete now produces one acl-bypass record per face instead of one per delete.
severity: minor
resolution: 'Kept on purpose: each face is a separate bypassed write. Documented in the authorizeFamilyDelete godoc.'
status: addressed
---
