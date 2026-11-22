---
id: RR-KGVTVLQ
type: review-response
title: Limit below 1 panics
finding: EntityChanges indexed out[len-1] for a zero limit.
severity: minor
resolution: 'EntityChanges refuses a limit below 1. Test: TestEntityChanges_FullPageStopsReading.'
status: addressed
---

EntityChanges indexed out[len-1] for a zero limit.
