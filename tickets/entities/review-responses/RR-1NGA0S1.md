---
id: RR-1NGA0S1
type: review-response
title: Stamp decodes JSON before the size and depth limits
finding: '[security] stampState fully decoded and re-encoded each state before upstream''s 1 MiB and depth-20 checks ran, so large or deeply nested states cost two JSON passes before being refused.'
severity: minor
resolution: stampState now refuses a state over 1 MiB or deeper than 20 levels before decoding, with a byte scan copied from the awareness package; table tests cover both.
status: addressed
---
