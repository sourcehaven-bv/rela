---
id: RR-HY0XGQ
type: review-response
title: Title-only relation fields skip the side check
finding: '{relation, direction} on the wrong side still validates and renders nothing.'
severity: minor
resolution: Pre-existing behaviour for title fields. Turning it into a load error can stop existing deployments from booting, so it needs its own ticket and a migration note.
reason: 'Pre-existing behaviour for title fields. Making it a load error can stop existing deployments from booting, so it needs its own ticket and a migration note.'
status: deferred
---
