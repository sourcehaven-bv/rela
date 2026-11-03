---
id: RR-E7G2CI
type: review-response
title: A failed busy read clears the wake and takes the board out of both live wakes and the safety replay
finding: 'internal/dataentry/busy_rows.go:54-61 returns before recordBusy on a store error, so the replay answers 200 without _busy and applyTopic (live_topics.go:210-215) sets busyWake=nil. From then on selectsBusy never picks the board and safetyDue (live_inbox.go:183) skips it, so the board gets no busy update until an unrelated graph write, lease change or reload starts a full round. On a quiet system that can be hours. A transient ListWaiting error is plausible on the file backend while another process (the PoC''s stdio rela mcp) writes channel files. That is the stale-board bug this ticket fixes. The wake comes from the gated rows and the configured type, not from the busy answer, so it is still valid after a store error. Fix: call recordBusy before the err check, or treat a store error like an unfinished replay that keeps the previous wake. Record it only for config and identity reasons (ok=false) that a reload or a graph write must change anyway. Add a TestLive_BusyWakeSetFromReplay row with a failing busyReader.'
severity: significant
resolution: 'Fixed: serveRowBusy records the wake from the gated rows before returning on a store error; a read not served for config or identity still records none. Test: TestLive_BusyWakeSetFromReplay/busy read fails, with a failing busyReader, asserts the wake holds and a later Mark replays the board.'
status: addressed
---
