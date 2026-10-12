---
id: RR-CRUYKL
type: review-response
title: Re-point from the target side of a max_incoming 1 relation always refused
finding: A max_incoming 1 relation could not be re-pointed from the target side through its inverse key. Every form refused the write.
severity: critical
resolution: Fixed in 21e383051. ReplaceRelations handles both sides. Covered by TestPatchRelations_RepointsMaxIncomingFromTarget and TestReplaceRelations_RepointsIncomingSide.
status: addressed
---

Review finding R1-2.
