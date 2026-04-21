---
id: RR-O4DUL
type: review-response
title: ETag does not cover outgoing relations
finding: 'computeEntityETag hashes ID/Type/Content/Properties only. PATCH can now change outgoing edges without changing the ETag, which poisons If-None-Match / If-Match round-trips: a client can GET with a stale ETag and receive 304 with outdated relations, or two relation-only PATCHes won''t conflict when they should. Also, the current ETag iterates map[string]interface{} which is non-deterministic across processes. Fold sorted relations into the hash and sort properties.'
severity: critical
resolution: computeEntityETag now sorts Properties keys and folds outgoing relations (sorted `type|to` tuples) into the hash. Relations-only PATCH changes the ETag, property-key map-iteration no longer causes non-determinism, and If-Match / If-None-Match round-trips stay honest. TestV1UpdateEntity_Relations_OnlyPATCH_ETagChangesButEntityStable asserts the ETag changes after a relations-only PATCH while entity fields are byte-stable.
status: addressed
---
