---
id: RR-GPI45X
type: review-response
title: Removed quiet messages break runs into separate fold lines
finding: AC 4 keeps every removed message as a row because it has no type (types.ts:203-204). In a run of many progress messages, one removed message splits the run into two fold lines with a lone placeholder between them. The kept-row reason (something the reader must act on) does not apply to a placeholder. Consider letting the app pass the original type for a removed message, or folding a removed placeholder that sits inside a run into that run. At least add it to the edge cases.
severity: minor
resolution: A removed message between two members of a run now joins it, counts in the fold line and shows as the placeholder when expanded, and removed messages are no longer a kept-row reason (AC 4). Added table cases and an edge case.
status: addressed
---
