---
id: RR-BTEXHHN
type: review-response
title: Leak failures printed only a count
finding: A failure printed only the number of goroutines, which does not separate a real leak from a profile-format change.
severity: minor
resolution: The failure messages now include the matching goroutine profile records.
status: addressed
---
