---
id: RR-LCQ63Z
type: review-response
title: Channel.typing duplicates config already served by the schema and _actions
finding: 'Step 12 adds Channel.Typing, which then appears in every head, page (MessagePage.channel) and list entry, although it is a channel type property, not a per-channel one. The SPA reads channelType.typing from the schema (step 18), and the decision to send is _actions.typing. Plan change: drop Channel.Typing, ChannelView.Typing and the matching golden and CHANNEL_WIRE_FIELDS churn. CLI and MCP can read ChannelTypeView.Typing from describe, or rely on actions.typing.'
severity: nit
resolution: 'Plan updated: Channel.typing and ChannelView.Typing are dropped with their golden and CHANNEL_WIRE_FIELDS churn; the schema endpoint and _actions.typing carry it; the CLI change is out of scope.'
status: addressed
---
