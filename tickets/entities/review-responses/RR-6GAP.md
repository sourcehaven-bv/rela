---
id: RR-6GAP
type: review-response
title: Vue comment implies one-way contract; it's actually three-way
finding: 'The new comment above CHECK_TYPES says keys must match section.Name. The full contract is three-way: runAnalysis() in Go produces the order, CHECK_TYPES renders that order, and ANALYSIS_CHECKS in e2e/tests/fixtures.ts asserts that order. Expand the comment to mention the e2e fixture and the order coupling.'
severity: nit
resolution: 'Expanded the comment above CHECK_TYPES in AnalyzeView.vue to spell out the three-way contract: runAnalysis() in Go produces sections in order; CHECK_TYPES keys match section.Name; e2e ANALYSIS_CHECKS asserts the same ordered list. Mentions TestRunAnalysisSectionNames as the Go-side guard.'
status: addressed
---
