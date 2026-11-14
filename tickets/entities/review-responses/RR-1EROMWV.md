---
id: RR-1EROMWV
type: review-response
title: No-tests output ignored -o json and named the wrong place
finding: With no tests the JSON format printed text, and the message always named tests/.
severity: minor
resolution: JSON prints an empty report; the message names the given paths. TestTestCmd_NoTests covers both.
status: addressed
---
