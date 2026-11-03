---
id: RR-2LP8EH
type: review-response
title: Addressed selection ignores kinds
finding: 'The MCP watch wakes on an addressed target only when kinds asks for mentions or replies (tools_watch.go:206-208, asksMentions). The hub rule replays a kinds=waiting or kinds=unread subscription on every mention as well. This is harmless (one extra replay, no frame), but it is cheap to match: keep a mentionKinds flag in inboxWake.'
severity: nit
resolution: inboxWake carries a mentionKinds flag from the request kinds, addressed targets only select subscriptions that ask for mentions or replies, and AC 9 checks a kinds=waiting subscription is not replayed.
status: addressed
---
