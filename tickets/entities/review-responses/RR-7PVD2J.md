---
id: RR-7PVD2J
type: review-response
title: 304 can serve a cached body with a released lease
finding: the ETag ignores the lease so If-None-Match after a release returns 304 for a body still showing _lease
severity: significant
resolution: 'Fixed in the review-fix commit (Lease HTTP: review fixes) with tests where behaviour changed.'
status: addressed
---
