---
id: RR-STCCY8
type: review-response
title: analyze all double-scans cardinality; error path untested at CLI boundary
finding: 'AnalyzeAllCmd.Run computes AnalyzeAll (one full cardinality scan) then runAnalyzeAllSections re-runs AnalyzeCardinalityCmd (a second full scan). Pre-existing waste, but the new error channel adds a consistency hazard: a flaky backend can pass the summary scan and fail the section scan, printing a contradictory report. Additionally no CLI-layer test covered the new error returns — the assertion that pins ''no output before the error'' was missing.'
severity: significant
resolution: 'Added TestAnalyzeCmds_CountErrorAborts (internal/cli/analyze_test.go): a failingCountStore wired through appbuildtest.WithStore + newCLIBundles asserts, for both `analyze cardinality` and `analyze all`, that the wrapped store error is returned and the output buffer is EMPTY. The double-scan itself is pre-existing and out of ticket scope; documented in-code at the runAnalyzeAllSections cardinality call (flaky-backend disagreement hazard, compute-once as the follow-up fix).'
status: addressed
---
