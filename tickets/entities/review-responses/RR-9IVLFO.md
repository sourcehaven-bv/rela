---
id: RR-9IVLFO
type: review-response
title: Add .testcoverage.yml floor for new internal/imgproc package
finding: New package internal/imgproc isn't covered by .testcoverage.yml floors; per CLAUDE.md the floor exists to catch a 'new untested package added', so security-critical error/edge paths could land under-covered without CI noticing.
severity: minor
resolution: Add an explicit package floor for internal/imgproc in .testcoverage.yml as part of this ticket. Noted in the plan's implementation-checklist prerequisites.
status: addressed
---
