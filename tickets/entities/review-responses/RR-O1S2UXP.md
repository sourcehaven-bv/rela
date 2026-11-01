---
id: RR-O1S2UXP
type: review-response
title: Query breakdown counts hidden values as Unknown
finding: queryBreakdown ignored _redacted, so a hidden group_by value fell into the unset group; the server skips such rows and marks the card partial.
severity: significant
resolution: queryBreakdown skips redacted rows and returns partial; CardView.partial drives the existing partial note for query cards too. Unit test with _redacted.
status: addressed
---
