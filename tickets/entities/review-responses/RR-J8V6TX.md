---
id: RR-J8V6TX
type: review-response
title: 422 error used the canonical relation name instead of the body key
finding: The 422 cardinality error named the canonical relation even when the client sent the inverse key.
severity: nit
resolution: Fixed in 21e383051. The error uses the key from the request body. Covered by TestPatchRelations_AddOverMaxIncomingNamesInverseKey.
status: addressed
---

Review finding R1-11.
