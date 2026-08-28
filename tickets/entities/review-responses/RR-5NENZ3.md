---
id: RR-5NENZ3
type: review-response
title: Ambiguity error used break inside a switch to skip the side check
finding: The ambiguous path appended its error then used `break` inside a switch inside an if, skipping validateFormRelationSide. Skipping is correct (without a direction there is no side to check), but `break` in that nesting is the construct that gets misread on the next edit — someone adds a statement after the if block expecting it to run.
severity: significant
resolution: 'Restructured to an explicit `ambiguous` boolean: the error is appended when ambiguous, and the side check is guarded on !ambiguous with a comment stating why it is skipped. No control-flow trickery, and the span/widget checks below still run in both cases.'
status: addressed
---
