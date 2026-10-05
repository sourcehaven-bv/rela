---
id: RR-QUXMAF
type: review-response
title: ACL gate arms become existential under AllFaces reads
finding: Under ruling D5 an ACL HasInbound/HasOutbound arm evaluated during an AllFaces or AtFaces read tests the endpoint at every selected face. That can grant more than evaluating the same gate in the request's world. The review recommends that gate arms always evaluate their endpoint in the request's world. This refines ruling D5 so it is left open for the coordinator.
severity: significant
resolution: 'Coordinator ruling (stage2-design section 12): ACL gate arms evaluate faced endpoints in the request''s world; only non-ACL EndpointMatch uses the query selection. Lands in PR 8 with a regression test.'
status: addressed
---

## Finding

Under ruling D5 an ACL HasInbound/HasOutbound arm evaluated during an AllFaces
or AtFaces read tests the endpoint at every selected face. That can grant more
than evaluating the same gate in the request's world. The review recommends that
gate arms always evaluate their endpoint in the request's world. This refines
ruling D5 so it is left open for the coordinator.

Design: `.ignored/stage2-design.md` section 11.
