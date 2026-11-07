---
id: RR-IX7IWMK
type: review-response
title: Range.Remove from empty range unspecified
finding: Count is uint64 so removing from an empty range underflows.
severity: minor
resolution: Documented that Count wraps mod 2^64 like Sum so Remove inverts Add; tested.
status: addressed
---
