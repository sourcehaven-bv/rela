---
id: RR-3DB4L4
type: review-response
title: MCP watch on read_channel fires on every typing refresh
finding: 'Step 9 adds TypingNow (with expires_at) to ReadView when req.Signals. The watch wakes on any mark of the key (internal/mcp/tools_watch.go:380-395) and reports a target when its digest moves. A refreshed expires_at moves the digest, so an agent that watches a channel with signals:true gets woken every 3 s while a human types. Each wake is an LLM turn, which is real money. Plan change: the stable {author, since} view plus marking only on start/clear/expiry fixes this. Add TestReadChannel_TypingRefreshDoesNotWakeWatch in internal/mcp next to the existing watch tests (watch returns once at start, not per refresh).'
severity: significant
resolution: 'Plan updated: typing_now carries no moving field and refreshes do not mark, so the read_channel digest changes only on start/stop; TestReadChannel_TypingRefreshDoesNotWakeWatch added (AC 20).'
status: addressed
---
