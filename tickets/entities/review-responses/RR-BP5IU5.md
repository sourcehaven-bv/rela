---
id: RR-BP5IU5
type: review-response
title: Inbox grid counts processed ticks, so slow rounds move its wall-clock phase
finding: 'internal/dataentry/live_inbox.go:72-78 (onInboxGrid/onInboxSafety) take the grid from h.ticks, which live.go:681 increments once per processed tick. time.Ticker drops ticks while the worker is busy. A round that takes 1.8 s (inside the AC 6 fs bound) therefore drops about 3 ticks, and every later grid and safety tick lands about 1.5 s later in wall-clock time. Such a round can be caused by a change the subscriber cannot see: a hidden graph write starts a full round, and a hidden mention or waiting-type message replays inboxes. Scenario: a hidden write at t0 makes the round at t0+0.5 s slow, so Alice''s visible mention at t0+1 s frames about 1.5 s later than it would without that write, and every frame after it stays shifted. The package doc at live_inbox.go:30-34 ("the grid ... never moves, so a hidden change can neither delay nor advance a visible frame") is then false, and the docs'' "within 15 seconds" safety bound (docs/realtime.md, docs/data-entry.md) assumes 500 ms ticks. AC 15''s test passes only because the fake ticker never drops a tick. Fix: derive the grid slot from the tick''s timestamp (the time.Time the ticker sends), for example replay when floor(t/(window*grid)) differs from the slot of the last inbox replay. Alternatively, drop the claim and state the bound in processed ticks.'
severity: minor
resolution: The grid slot now comes from the tick timestamp since the hub's first tick; skipped periods carry their safety slices forward. TestInboxClock_TickAt and TestLive_InboxGridSurvivesDroppedTicks.
status: addressed
---
