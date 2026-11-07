---
id: RR-G9TT1JZ
type: review-response
title: 'Code: fmt.Errorf without verbs'
finding: pgstore used fmt.Errorf with no format verbs.
severity: nit
resolution: The code moved to store.RangeSumFromAggregates, which uses errors.New.
status: addressed
---
