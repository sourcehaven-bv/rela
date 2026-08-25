---
id: RR-TFUP2X
type: review-response
title: Pinning suite missed the min/max interleaving regression; test import nit
finding: 'None of the pinning tests would fail if the two emit passes were collapsed into a single count-and-emit pass: catching that requires a min AND a max violation across two subject types (grouped [A-min, B-min, C-max] vs interleaved [A-min, C-max, B-min]). Also the OrderingAndLabels godoc overclaimed (''the full per-relation contract''), and analysis_test.go aliased `stderrors "errors"` with no name collision to justify it.'
severity: minor
resolution: Added TestCheckCardinality_MinMaxGroupedAcrossTypes (two From types, min+max violations, asserts the grouped order a single pass would break); trimmed the OrderingAndLabels godoc claim and cross-referenced the sibling tests; switched the test file to a plain `errors` import.
status: addressed
---
