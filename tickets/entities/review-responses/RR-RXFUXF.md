---
id: RR-RXFUXF
type: review-response
title: Any channel wake in the process replays the full inbox
finding: 'internal/mcp/tools_watch.go:178 (`c.all ||` in matches) plus inboxDue (:516-519), which returns true for passWake: every chat post, edit or reaction in any channel of the process replays every list_inbox watcher at the next 500 ms tick. Each replay costs pairs x holder query + ListWaiting + anchor batch + prime. With 32 watches and one busy room, that is tens of full inbox computations per second. A change in a channel type that has no waiting state can never change an inbox. Fix: give the inbox watchChannel the set of channel types from Config.Waiting().States and match on type. Optionally apply a short minimum interval (1 to 2 s) to wake replays as well.'
severity: minor
resolution: A list_inbox target wakes only for channel types with a waiting state; a mutation check shows the test catches the extra replays
status: addressed
---
