---
id: RR-F45AS7
type: review-response
title: ChannelService consumer interface keeps growing (8 methods, T4c adds more)
finding: internal/dataentry/channels_handler.go:33-43 is far past the 1-3 method guidance in CLAUDE.md, and T4c will add RunAction/ReleaseAction. Split it now into a read interface (ListAnchor/Head/Messages) and a write interface, or per-route narrow interfaces, before T4c doubles down.
severity: minor
resolution: 'Replaced the exported ChannelService with three unexported consumer-side interfaces: channelReader (ListAnchor, Head, Messages), messageWriter (Post, Edit, Delete) and channelWriter (Create, Lifecycle). channelsHandler holds one field per capability, and SetChannels passes the service to each.'
status: addressed
---
