---
id: RR-KLEV6K
type: review-response
title: A seq-only change to a bad seq keeps the previous pending reveal
finding: 'frontend/src/components/entity/channels/ChannelsSection.vue:178: when only seq changes to an invalid value while a reveal is still pending (the channel has not loaded yet), the early return leaves the old {key, seq} in place, so the old target is revealed for a link that asked for nothing. Set pendingReveal to null when seq is invalid but present, and keep the early return only for an absent seq (our own replace).'
severity: nit
resolution: ADDRESSED a seq-only change to a present but invalid seq sets pendingReveal to null; only an absent seq returns early. Test "a bad seq drops a pending reveal".
status: addressed
---
