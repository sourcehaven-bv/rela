---
id: RR-C08IW4U
type: review-response
title: Two blocks read the same channel list twice
finding: Each block calls useEntityChannels on the same anchor, so a page with two blocks sends two identical list requests and holds two live keys on the same path.
severity: minor
resolution: Deferred; the budget is documented and tested at two keys per block.
reason: Within the stated budget (worst stream 13 of 16). Sharing the list needs a provider that keeps the per-block rule that a block drops its list key while a chosen channel is open; that is a refactor of useEntityChannels, not needed for this feature.
status: deferred
---
