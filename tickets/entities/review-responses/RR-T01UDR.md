---
id: RR-T01UDR
type: review-response
title: Plan says empty value is rejected
finding: --access-log= is accepted as off by flag parsing; only the constructor rejects empty.
severity: nit
resolution: Plan wording corrected; TestCheckAccessLogDest pins empty as off at flag level.
status: addressed
---
