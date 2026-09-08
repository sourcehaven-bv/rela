---
id: RR-LT2O71
type: review-response
title: ConditionPrefilters derived Scalar from a condition no reachable input exercised
finding: 'Scalar: !eq.List && value != "" — the value check was dead for both branches (empty identity continues earlier; empty literal refused by ConstEqualities), so it read as if empty values were expected.'
severity: nit
resolution: 'Simplified to Scalar: !eq.List with a comment recording why an empty value cannot reach the predicate.'
status: addressed
---
