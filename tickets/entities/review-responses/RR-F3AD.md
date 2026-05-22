---
id: RR-F3AD
type: review-response
title: New tests live under 'click discrimination' describe block
finding: The three new tests are rendering tests, not click-discrimination tests. They should live in a sibling describe('AnalyzeView section rendering'). Cosmetic, but improves test report clarity and future bisect localisation.
severity: nit
resolution: The three new tests live in a sibling describe('AnalyzeView section rendering (GH#785)') block, separate from the existing click-discrimination block.
status: addressed
---
