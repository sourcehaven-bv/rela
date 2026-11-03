---
id: RR-3ZXDOY
type: review-response
title: File-store replay cost has no bound and no acceptance criterion
finding: 'Every connected user holds one summary subscription for the whole session, plus one inbox subscription while the page is open. Every graph write starts a full round that replays all of them. On fs each replay costs 2-3 walks over all channel directories plus per-anchor visits. So the cost per round is N users x 2 shapes x O(channels), it shares the 4 replay slots (liveParallel) and the 10 s round budget with every other live subscription, and slow inbox replays push entity, list and channel subscriptions into fallback refetches. The plan only says "measure" and leaves the full-round cadence open. Ask for: (1) an AC with a number, for example a round with 20 inbox subscriptions on the perf project at 1000 channels stays under X ms on fs and on postgres; (2) one Service.Inbox call per principal per round, shared by the summary and page replays of that principal (a per-round memo keyed by ShareKey, never shared across principals); or, simpler, while the page is mounted useInboxSummary takes count, truncated, unread_count and mention_count from the page GET and drops its own subscription; (3) a decision on the full-round grid before implementation, not after measuring.'
severity: significant
resolution: 'Added a per-round per-principal memo of Service.Inbox (AC 5: 20 principals with 80 subscriptions make exactly 20 calls), a numeric round bound on the perf project (AC 6: under 2 s on fs, under 500 ms on postgres), and fixed the grid decision before implementation with a pre-review fallback.'
status: addressed
---
