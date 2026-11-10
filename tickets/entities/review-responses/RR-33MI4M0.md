---
id: RR-33MI4M0
type: review-response
title: 'Error repeats the tenant: prefix'
finding: 'The refusal reads ''tenant: unknown organization: tenant: org_id is required''.'
severity: nit
reason: The same wrapping is used for every ErrUnknownTenant in the package; the message reaches only logs and the operator CLI.
status: wont-fix
---
