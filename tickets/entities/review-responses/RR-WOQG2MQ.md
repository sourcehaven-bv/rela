---
id: RR-WOQG2MQ
type: review-response
title: No timeout on a tenant open
finding: context.WithoutCancel removes the request deadline, so a hanging connect or migration blocks Acquire without limit.
severity: minor
reason: Migration time has no safe fixed bound\; open lifecycle and single-flighting belong to the provisioning work (TKT-TNPRV8).
status: deferred
---
