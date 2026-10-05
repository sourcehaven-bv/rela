---
id: RR-M2OAAF
type: review-response
title: 'PR 8: inheritance-closure changes untested'
finding: The per-id entity-closure seed and the identity-tail filter on both closures had no storetest case, and the differential fixture had no faced-only family or content-tailed closure edge.
severity: significant
resolution: Added storetest InheritanceClosures (faced-only candidate inherits through an identity edge; draft-tailed edges neither confer nor expand), passing on mem, fs, sqlite and pg. The differential fixture gained faced-only ITM-FO and a draft-tailed childOf edge.
status: addressed
---
