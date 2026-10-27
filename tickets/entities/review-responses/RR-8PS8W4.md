---
id: RR-8PS8W4
type: review-response
title: A mark broadcasts a change to every subscriber of the channel, which is N-squared replays per message
finding: 'Service.MarkRead calls s.local.mark(c.key). Local.mark adds the key to every Subscription in the process. The live hub then replays the head of that channel and the channel list of its anchor family for every principal subscribed, and every list_inbox watch whose types include the channel''s type replays its whole inbox. With N viewers, one new message causes N marks (one per tab within about 1 s), and each mark causes N head replays. Each replay is a Head (which takes the global fs lock) plus a Reads. Only the marker''s own frame changes; the plan acknowledges that the others are wasted work. Fix: drop the broadcast. The PUT and mark_read answers already carry {seq, unread, last_seq}. If other tabs of the same reader must update live, add a reader-scoped wake (for example Local.markFor(reader, key) that only subscriptions bound to that reader take). Pin it with a test: one mark by A replays 0 subscriptions of B.'
severity: significant
resolution: 'No broadcast. Moved marker calls Local.markFor(reader, key); subscriptions carry a reader via SubscribeAs. TestLive_MarkReadFramesHead: A''s mark replays 0 of B''s subscriptions.'
status: addressed
---
