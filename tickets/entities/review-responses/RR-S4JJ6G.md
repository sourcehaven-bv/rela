---
id: RR-S4JJ6G
type: review-response
title: Target cap changed forward links and logged on every parse
finding: Forward mention links went through the capped Targets, and the cap warning fired on every parse, including each memstore read.
severity: minor
resolution: 'internal/mentions now has All (uncapped, forward links), Targets (capped, quiet) and Index (capped, warns once per persisted write). Test: TestAll_Uncapped.'
status: addressed
---
