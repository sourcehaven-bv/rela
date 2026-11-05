---
id: RR-I4B3MIA
type: review-response
title: A served record that lands after a round's commit misses its round
finding: 'The first plan recorded the hash after serving and compared it at the next round that happened to replay the sub. A record landing after round N committed the reverted state waited for a round that nothing dirtied: the same bug through a smaller window.'
severity: critical
resolution: A served hash that differs from the baseline puts the sub in pending (noteServedLocked) so the next tick replays it. Pinned by TestLive_ServedAfterCommitFrames.
status: addressed
---
