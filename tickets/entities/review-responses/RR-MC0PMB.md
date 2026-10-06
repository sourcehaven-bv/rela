---
id: RR-MC0PMB
type: review-response
title: Untypeable peer still blocks the save although an untyped fallback exists
finding: For link_as=to with an inverse, a prefix-less peer id still aborted the create, while the post-create incoming call needs no peer type.
severity: significant
resolution: linkBodyKey returns undefined when the peer type does not resolve, so such a peer is linked after create with direction incoming. New unit test covers it.
status: addressed
---
