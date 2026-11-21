---
id: RR-W06MC83
type: review-response
title: Status answers could tell which orgs exist
finding: 'Security review: a 404 for an unknown org next to a 200 for a known one would let a caller enumerate tenants.'
severity: minor
resolution: 'Addressed: the token check and the org binding run first, so only a holder of a valid token for that org sees the status. TestTenantAPI_Auth pins 401 and 403 before any lookup.'
status: addressed
---
