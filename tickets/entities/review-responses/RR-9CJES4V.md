---
id: RR-9CJES4V
type: review-response
title: Conditions must stay per relation type
finding: conferredEdge folds all conferring types into one OfTypes predicate, so one EdgeProps would apply to all of them.
severity: significant
resolution: 'Plan: each conditional relation type gets its own predicate branch; EdgeProps only on the conferring edge.'
status: addressed
---
