---
id: RR-AUQGSV
type: review-response
title: Metamodel-derived traversal indexes are scope creep
finding: Deriving indexes from schema.yaml surfaces adds a second desired-state input without the partial-input guard and without EXPLAIN tests; the one-row surfaces gain little.
severity: significant
resolution: 'Dropped: indexes are derived only from view and next-action condition traversals (data-entry.yaml; same guard); with EXPLAIN tests on both backends; or deferred to a follow-up if the shapes need new SQL.'
status: addressed
---
