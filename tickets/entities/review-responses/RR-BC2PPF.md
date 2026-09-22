---
id: RR-BC2PPF
type: review-response
title: 'Conformance suite could not catch the drift it was written to prevent'
finding: 'RunBulkMigrateTests asserted only final row state and counts, so it was green while three backends emitted three different event streams, while updated_at never moved, and with no faced-edge case exercised at all. The suite''s own doc comment cites BUG-TMGWIN - where detection and remediation were maintained as separate lists, each looking complete alone - as the reason it exists, which made the gap the least defensible one in the change.'
severity: significant
resolution: 'The suite now asserts the event pair and updated_at movement, and a cycle case (A->B->C->A) was added: that is where a naive loop can rewrite into a key it has not read yet, and each edge must keep its own payload rather than inherit a neighbour''s. All four backends pass. The faced-edge path is now refused by the store itself rather than left untested-and-reachable.'
status: addressed
---
