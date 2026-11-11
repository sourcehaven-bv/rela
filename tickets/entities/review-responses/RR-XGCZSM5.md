---
id: RR-XGCZSM5
type: review-response
title: The conflict list is one document that grows with held entries
finding: Each entry keeps up to 16 KiB of remote value; many held fields on many rows make one large state.KV document read and written each run.
severity: minor
reason: Bounded by max_rows times mapped fields and the 16 KiB cap; held entries need a person anyway. Revisit with the step 10 status page if lists grow in practice.
status: deferred
---
