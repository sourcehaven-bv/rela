---
id: RR-SZHVAD
type: review-response
title: list_inbox watch replays on every in-process channel wake, unpaced, sharing the server-wide replay slots
finding: 'internal/mcp/tools_watch.go:731 (watchChannel.all matches every channel key) plus inboxDue at tools_watch.go:893 returns true for passWake whenever the previous replay finished, so any post on any channel in the process (by any user, on remote MCP) triggers a full inbox recompute for every list_inbox watcher at each 500 ms tick. One inbox replay is one holder query per waiting pair (paged up to 20001 gated, redacted rows each), a ListWaiting of up to 20000 rows and a batch anchor read. inboxPollEvery paces only the poll path. The four watch slots (watchParallel) are per watchHandler, which the stateless remote MCP handler shares across all principals, so a few inbox watchers on a busy server can keep the slots occupied and starve other users'' read_channel/show_entity watches. Fix: apply the inbox pacing (a minimum interval, e.g. a few seconds) to passWake as well as passPoll, or have the realtime waker deliver only keys whose type is in WaitingIndex.States, and give list_inbox replays a separate, smaller slot budget.'
severity: minor
resolution: Wake and store-event inbox replays run at most once per 2 s with a trailing run; together with the waiting-type filter this bounds the load (21 replays down to 1-4 in the test)
status: addressed
---
