---
id: RR-PDJV2Y
type: review-response
title: POST relations hard-422s on a mismatch
finding: handleV1CreateRelation passes no flag, so POST rejects what PATCH writes with a warning. Predates this change.
severity: minor
reason: 'Pre-existing inconsistency outside #1806; filed as a follow-up ticket.'
status: deferred
---
