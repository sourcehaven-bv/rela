---
id: RR-0JGGEY
type: review-response
title: Hidden writes move frame timing
finding: 'Security and code review: any feed wake opened the 500 ms window. A write to a type the subscriber cannot read made a visible change frame earlier. docs/acl-security.md claimed polling gives the same timing.'
severity: significant
resolution: Rounds run on a fixed 500 ms ticker when anything changed. A hidden write no longer moves when a frame arrives. The overload fallback (cap or time budget) can still frame an unchanged response after a hidden write; acl-security.md now states that residual instead of the incorrect polling claim.
status: addressed
---
