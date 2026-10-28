---
id: RR-JVY411
type: review-response
title: One failing card blanked the whole dashboard
finding: Card fetches ran under Promise.all, so a 400 on one list card set the page-wide load error.
severity: significant
resolution: 'Each list card fetch catches its own error into cardErrors; only that tile shows "This card could not be loaded". Test: fails one card without blanking the dashboard.'
status: addressed
---
