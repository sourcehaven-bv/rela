---
id: RR-MLB7OM
type: review-response
title: 'Smaller notes: VerifiedFrom doc, MapWebhook resolution, permission universe'
finding: VerifiedFrom doc should name the self-minted exception; MapWebhook must not resolve principal_property; reuse permission lists.
severity: nit
resolution: VerifiedFrom doc names the exception. MapWebhook uses the raw name. The permission universe is the union of all roles' permission lists, since "*" matches no named permission.
status: addressed
---
