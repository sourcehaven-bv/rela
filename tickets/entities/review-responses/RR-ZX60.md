---
id: RR-ZX60
type: review-response
title: e2e fixture ANALYSIS_CHECKS comment is stale and misleading
finding: 'The comment block above ANALYSIS_CHECKS in e2e/tests/fixtures.ts says the spec will fail if CHECK_TYPES grows, but doesn''t mention: (a) Playwright''s toContainText array form is order-sensitive, so ANALYSIS_CHECKS order is load-bearing; (b) the real source of truth is runAnalysis() in internal/dataentry/analyze.go, not CHECK_TYPES; (c) all three must stay in lockstep. Update the comment to cite analyze.go and call out order coupling.'
severity: minor
resolution: 'Rewrote the comment block above ANALYSIS_CHECKS in e2e/tests/fixtures.ts: cites runAnalysis() as the real source of truth, calls out Playwright''s order-sensitive toContainText array form, and mentions TestRunAnalysisSectionNames as the canonical Go-side guard.'
status: addressed
---
