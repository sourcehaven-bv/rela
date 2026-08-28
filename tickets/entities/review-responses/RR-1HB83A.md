---
id: RR-1HB83A
type: review-response
title: Count error message did not name the direction
finding: The wrapped error named entity and relation, but each (entity, relation) pair is counted twice — outgoing and incoming — and the message could not distinguish which count failed. In a partial backend failure that distinction is the diagnosis.
severity: minor
resolution: 'Error now reads `analysis: count <outgoing|incoming> "rel" relations of <id>: ...`, derived from spec.direction.'
status: addressed
---
