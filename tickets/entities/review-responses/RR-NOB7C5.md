---
id: RR-NOB7C5
type: review-response
title: '[security] Authorization cache keyed by face only'
finding: The per-delete authorized set was keyed by face alone; the ACL subject also includes the type.
severity: nit
resolution: familyAuthorization is keyed by (type, face).
status: addressed
---
