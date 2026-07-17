---
id: RR-QD5BSI
type: review-response
title: peerIDs not deduplicated in handleV1GetRelationType (harmless inconsistency with neighborIDsOf)
finding: 'peerIDs is collected without dedup, unlike the sibling neighborIDsOf which dedupes via a seen map. Two edges to the same peer add the ID twice. Harmless: filterVisible groups by type, PermitsReadMany takes a set-like slice, and the result map is ID-keyed so dupes collapse.'
severity: nit
reason: Deferred with the double-walk nit to the TODO(BUG-ABXMAV) chokepoint follow-up. Non-blocking (nit). A one-line comment or reusing a dedupe helper would remove the inconsistency.
status: deferred
---
