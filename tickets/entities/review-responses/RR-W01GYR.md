---
id: RR-W01GYR
type: review-response
title: 'Nits: second config load, stale godoc, test names'
finding: maybePrune reloaded the config; StartBackground godoc said no-op; a test name overclaimed; a duplicate assertion.
severity: nit
resolution: maybePrune takes the tick's cfg; godoc rewritten; test renamed; assertion removed; CLI behavior documented.
status: addressed
---
