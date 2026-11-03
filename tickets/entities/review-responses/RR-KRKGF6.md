---
id: RR-KRKGF6
type: review-response
title: Non-browser clients can claim but not renew or release
finding: The claim route is non-browser exempt but /api/v1/_leases/ is not; token clients strand leases until TTL and the OpenAPI text promises renew and release (also raised by the security review)
severity: significant
resolution: 'Fixed in the review-fix commit (Lease HTTP: review fixes) with tests where behaviour changed.'
status: addressed
---
