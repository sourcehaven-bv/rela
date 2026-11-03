---
id: RR-GH53XP
type: review-response
title: Skipped presence replays are never rescheduled
finding: fallback only marks the key changed; with no further writes no round re-replays the skipped presence sub so a revoked viewer stays listed to others until disconnect (fail-open state drift flagged by the commit security check)
severity: significant
resolution: 'Fixed in Presence: reschedule skipped replays. fallback queues skipped presence subscriptions; n skipped pairs are re-checked within ceil(n/256) ticks; TestLiveHub_SkippedPresenceReplayIsRescheduled fails without the fix.'
status: addressed
---
