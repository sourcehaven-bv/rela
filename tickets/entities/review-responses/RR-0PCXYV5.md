---
id: RR-0PCXYV5
type: review-response
title: 'Huge order values decode wrongly'
finding: 'order values past float64 precision were accepted with a changed value, and 1e300 got the wrong message.'
severity: minor
resolution: 'decodeWholeNumber refuses values beyond 2^31 with the whole-number error and the line; tested.'
status: addressed
---
