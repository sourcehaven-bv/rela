---
id: RR-3QL5
type: review-response
title: Cross-language key contract enforced only by a comment
finding: Strings 'Duplicates' and 'ID Gaps' in frontend/src/views/AnalyzeView.vue CHECK_TYPES must match section.Name in internal/dataentry/analyze.go exactly. The Vue comment documents this but nothing on the Go side fails if a section is renamed - the page silently regresses to the GH#785 bug. Add a Go test that pins the ordered list of section names and references the frontend constant in its failure message.
severity: significant
resolution: Added TestRunAnalysisSectionNames in internal/dataentry/analyze_test.go pinning the ordered list of section names produced by runAnalysis(). Failure message cites the SPA and e2e consumers and tells future Go-side renamers to update them in lockstep. Verified the test passes.
status: addressed
---
