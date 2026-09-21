---
id: RR-INVERR
type: review-response
title: An incoming edge on a relation with no declared inverse fails with a misleading error
finding: '"resolveDirection returns a bare ok=false for every unresolvable key and both callers map it to unknown_relation_type; the documented no_inverse_defined code is never produced. The read endpoint emits a synthetic <type>_inverse key for a relation with no declared inverse, so a duplicate carrying that key fails at create with ''relation type is not defined in the metamodel'' — naming a key the server itself just handed the client. Pre-existing, but this feature makes it reachable from the UI for the first time."'
severity: minor
reason: 'Deferred. Pre-existing: resolveDirection returns a bare ok=false and both callers map it to unknown_relation_type, so the documented no_inverse_defined code is never produced. Out of scope for this ticket; worth its own bug since this feature makes it reachable from the UI.'
status: deferred
---

## Suggested resolution

Out of scope for this ticket; file separately. Recorded so the next person does not rediscover it.
