---
id: RR-TDV1B2D
type: review-response
title: Late fill ignores userTouched and null
finding: The emptiness check refilled a field the user cleared and treated null and '' differently.
severity: minor
resolution: 'The late fill skips userTouched fields and uses isClearedForType. Test: does not refill a field the user cleared.'
status: addressed
---
