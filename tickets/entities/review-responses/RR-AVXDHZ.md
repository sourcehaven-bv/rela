---
id: RR-AVXDHZ
type: review-response
title: Block-type channel created in the guard window is moved, not refused
finding: channelguard.go:139 judges block on the Scope list. A block-type channel created before the Tombstone is filed as detach (kept, so it fails safe) instead of refusing the delete. A Filer returning (Filing, error) would let the store abort the move so the guard returns AnchorBlockedError.
severity: minor
resolution: The delete filer re-checks the block rule over the channels it moves and returns AnchorBlockedError (TestChannelGuard_LateBlockingChannelRefuses).
status: addressed
---
