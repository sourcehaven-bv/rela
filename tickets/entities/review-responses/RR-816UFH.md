---
id: RR-816UFH
type: review-response
title: The negative ACL and cost tests have no tick barrier and count replays by URL only
finding: 'newBusyLive runs the wall-clock 20 ms window (live_test.go:56, 768-775), not the manual ticker. An assertion that a board was replayed zero times (AC 5 every subtest, AC 6 "face not on the page", AC 7 for bob''s frames) therefore passes even when no round has run yet. AC 5 and the AC 6 face subtest also have no positive control in the same round. The existing replayCounter keys by URL alone (live_channels_test.go:49-53), so it cannot tell alice''s replay of `/api/v1/tickets?busy=run` from bob''s. Change the test plan: build the busy live env on the manual ticker, as newInboxLiveEnv does (env.ticks.step, live_inbox_test.go:925-960), and count with countPrincipalReplays (live_inbox_test.go:143-158). Make every negative assertion follow a stepped tick that, in the same round, frames or replays a positive control: the run channel subscription, as the old test does (live_test.go:828-830), or alice''s board. With the manual ticker, AC 10 can assert exactly one replay per burst instead of "at most twice".'
severity: significant
resolution: 'Adopted: the busy live tests run on a new manual-ticker env like newInboxLiveEnv and count with countPrincipalReplays, and every negative assertion follows a stepped tick with a positive control in the same round. AC 10 now asserts exactly one replay per burst.'
status: addressed
---
