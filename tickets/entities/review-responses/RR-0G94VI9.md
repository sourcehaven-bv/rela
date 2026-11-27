---
id: RR-0G94VI9
type: review-response
title: Tests miss decimals, removal and new edges
finding: No test for decimal storage, relation_removed or a rule depending on the edge being written; require.Empty on audit proved nothing.
severity: minor
resolution: Added those three cases on memstore and sqlite; audit asserted with per-op counts.
status: addressed
---
