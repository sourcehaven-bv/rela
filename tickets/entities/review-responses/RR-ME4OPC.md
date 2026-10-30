---
id: RR-ME4OPC
type: review-response
title: Inbox-named clock now drives busy boards
finding: 'The busy safety replay reuses inboxClock, inboxSlice, inboxSliceOf, inboxSafety, liveInboxSafetyMax and SetInboxGrid (live_inbox.go:79-183), and the busy test env pins inboxSliceOf (live_busy_test.go:50). Readers of live_busy.go must know that the inbox clock is the shared safety clock, and a later inbox-only change to the grid or the slices will silently change busy-board latency and cost. Fix: rename the shared parts to safetyClock, safetySlice, liveSafetyMax and so on (or move them to live_safety.go), keeping only the inbox grid deferral inbox-specific. A follow-up ticket is acceptable if the rename is too wide for this PR.'
severity: minor
resolution: 'Fixed: renamed the shared parts to safetyPeriod, safetySlices, safetySlice, safetySliceOf and liveSafetyMax. The live_inbox.go package comment now says that the safety clock and its grid are shared with busy boards and that only the grid deferral is inbox-specific. inboxClock, inboxTick and SetInboxGrid keep their names, since the grid is still configured as the inbox grid by the wiring.'
status: addressed
---
