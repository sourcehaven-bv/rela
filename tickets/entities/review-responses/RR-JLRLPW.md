---
id: RR-JLRLPW
type: review-response
title: EXPLAIN acceptance test is brittle
finding: 'A selective related() should drive the plan from relations_type_*_idx, the derived entity index serving ordering; listIndexSpec returns nothing without sort: (queryplan.go:480).'
severity: minor
resolution: 'Plan: AC5 asserts no Seq Scan on entities for a sorted list config; does not require a named derived index in the count query.'
status: addressed
---
