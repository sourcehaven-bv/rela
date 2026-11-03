---
id: RR-2Y2QVE
type: review-response
title: Lease POST routes must stay CSRF-protected
finding: 'Security design review: agents without Origin may tempt an exemption.'
severity: nit
resolution: 'Plan: never exempt /api/v1/_leases/; AC13 through NewRouter with cookie and foreign Origin.'
status: addressed
---
