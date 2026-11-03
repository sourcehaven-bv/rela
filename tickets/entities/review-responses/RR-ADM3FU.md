---
id: RR-ADM3FU
type: review-response
title: TestLive_TypingFrames runs ~8s of wall clock
finding: 'internal/dataentry/live_channels_test.go:369-418 sleeps TypingRate.Per twice and then waits for the real TTL (6s), because Local''s AfterFunc uses wall time. It is the slowest test in the package and the 500ms early-frame window is timing-sensitive under -race on a loaded CI runner. Fix: give the env a Local built with a fake clock and a short TTL seam, or move the timing assertion to internal/realtime (TestTyping_TimerWakes already covers the timer) and keep the live test to ''one frame at start, one at expiry'' with a generous deadline and no lower bound.'
severity: nit
resolution: TestLive_TypingFrames runs on an injected fake clock (appbuildtest.WithChannelsLocal), about 1.6s, no sleeps or wall-clock windows.
status: addressed
---
