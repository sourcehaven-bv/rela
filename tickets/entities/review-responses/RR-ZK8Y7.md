---
id: RR-ZK8Y7
type: review-response
title: SearchPage.openSelectedResult duplicated — TS2393
finding: Two implementations defined in search.page.ts; second silently wins at runtime and the docstring on the first is a lie (actually waits for DOM load).
severity: critical
resolution: First duplicate removed; single implementation left that waits for DOM load after Enter. Verified by typecheck.
status: addressed
---
