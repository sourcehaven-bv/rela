---
id: RR-PMRMT1
type: review-response
title: Live wake by channel type replays every busy board on every write to any channel of that type
finding: 'Plan step 5 selects a busy list subscription when f.types[busyType] is set. f.types is filled from TakeDurable (internal/dataentry/live_inbox.go:306-313), which returns every channel a stored write changed (internal/realtime/local.go:171-175): every message append, including the PoC runner''s frequent progress and text posts, not only session moves or closes as AC 7 says. It also fires for channels on anchors the subscriber cannot see. Each selected replay is the whole list pipeline (graph query, edges, owners, leases, rollups, includes) plus ListWaiting, which on the file backend is one directory walk per row (internal/channels/filechannels/ops.go:1220-1224). The risk section''s claim that a replay is one list read with one ListWaiting understates this. The plan should record the page''s anchors (or channel keys) in topicProbe, as inboxWake.keys does, and select a subscription only when a durable key''s anchor is on its page. Otherwise it should move AC 7 to a follow-up, since every PoC move already starts a full round through its graph outputs.'
severity: significant
resolution: Resolved by moving live board wake on channel changes (old AC 7) to the follow-up "busy board live refresh on session moves"; channel changes no longer select list subscriptions. The board still refreshes live on graph writes, and AC 7 now pins only that `busy` is an accepted live key and that a channel round does not select the board.
status: addressed
---
