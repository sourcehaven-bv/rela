---
id: RR-3Q1SV5A
type: review-response
title: Minor robustness in body-task roll-ups
finding: Quadratic dedupe in readBodies; TaskCount default fell through to total; compileColumn relied on the loader alone.
severity: minor
resolution: Seen-set dedupe; explicit total case; Compile re-checks count and type.
status: addressed
---
