---
id: RR-0Y4N3C
type: review-response
title: The 2 s inbox gap is anchored at the last replay, which is a timing oracle
finding: 'Step 6 sets inboxNext = now + 2 s whenever an inbox replay runs. Hidden changes start replays too: a full round from a graph write to a type the caller cannot read, a change in a waiting-type channel the caller cannot read, or a mention of the caller in a channel the caller cannot read (addressed targets are not gated). Such a replay pushes the next visible frame back by up to 2 s. Alice can then tell from when her mention frame arrives (t+0.5 s or t+2 s) that a hidden write or hidden mention happened in the previous 2 s. live.go:569-578 states the rule this breaks: a round''s start must never depend on when a change arrived, because a hidden write would otherwise move when the next frame arrives. The plan''s security claim ("timing shows no more than polling every tick would") is therefore false. Fix: put the gap on a fixed grid instead. Inbox subscriptions replay only on ticks where h.ticks % 4 == 0, for every round kind, and a selected subscription waits in pending until the next grid tick. This also removes the per-liveSub inboxNext field (which is wrong anyway, because replays are shared per (principal, url) pair, not per subscription) and the fake-clock test. Add a test: a hidden change at t0 does not change when a visible change at t0+1 s frames.'
severity: critical
resolution: Replaced the per-subscription gap with a global grid (inbox replays only on ticks where ticks % 4 == 0, for every round kind, deferred subscriptions wait in pending without a fallback frame), removed inboxNext and the fake-clock test, and added AC 15 (a hidden change at t0 does not shift a visible frame at t0 + 1 s).
status: addressed
---
