---
id: RR-GNM1
type: review-response
title: fmtfor reinvents fmt.Sprintf
finding: internal/workspace/orderable_test.go:46-54 hand-rolls a single-substitution string formatter to avoid importing fmt. fmt is already imported across the test suite. Replace with fmt.Sprintf(orderableMetamodelTemplate, suffix) and delete fmtfor.
severity: significant
resolution: Replaced fmtfor with fmt.Sprintf and added the fmt import. The hand-rolled helper is gone.
status: addressed
---
