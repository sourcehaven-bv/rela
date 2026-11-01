---
id: RR-Z2T4LL1
type: review-response
title: Per-face violations become duplicate issues
finding: 'Design review: a rule that fires on two faces of one family reports the bare id twice once violators resolve in the world.'
severity: nit
reason: Same behaviour as develop, where GetAddress resolved the bare id in the world too. Out of scope for a performance fix; it belongs with the candidates-once-per-type follow-up TKT-VJ8FHJF.
status: deferred
---
