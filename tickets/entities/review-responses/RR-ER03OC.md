---
id: RR-ER03OC
type: review-response
title: archplan's step summary published security finding details
finding: archplan ci appends a markdown summary with finding titles and file:line to GITHUB_STEP_SUMMARY by default. On a public repo that put the weekly trace-based findings and the base export's findings on public run pages. Raised by both reviewers.
severity: critical
resolution: 'The weekly run and the base export pass --summary to a file under RUNNER_TEMP. The weekly step summary gets one count line. The PR head run keeps its summary: it lists findings in the PR''s own public code and the ::warning annotations carry the same text anyway.'
status: addressed
---
