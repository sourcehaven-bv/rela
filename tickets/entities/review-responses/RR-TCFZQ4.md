---
id: RR-TCFZQ4
type: review-response
title: A pending promote cannot be released from the web app
finding: '§2 `api/channels.ts` and §5 omit `DELETE …/messages/{seq}/_action` (releaseAction, channels_handler.go:449). A promote that failed half-way leaves the message `pending`, which refuses every edit, removal and action until a moderator releases it (MessageAction godoc). Without UI the only fix is the CLI or MCP. Fix: show the pending status on the message, and a "Release" button for callers with `remove` (or whatever affordance the server exposes for release). Add it to the client, an AC and RlChannelMessage.test.ts.'
severity: significant
resolution: The client gets releaseAction, and a pending message shows its status and a Release button when release is in its _actions (R15, AC 18).
status: addressed
---
