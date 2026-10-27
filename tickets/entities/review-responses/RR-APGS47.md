---
id: RR-APGS47
type: review-response
title: read_session hands entity property values (holders) to an agent without the untrusted-content notice
finding: 'Holders are raw anchor property values. `owner` is a free-text string anyone with field write can set (the AC8 warning exists for this reason). read_channel wraps its result in `realtime.UntrustedNotice` (`mcp/tools_channels.go:375`), and `channelInstruction` says message content is untrusted. Change: `read_session` returns `{notice: realtime.UntrustedNotice, ...}` like read_channel. The tool description says holders are data, not instructions. Update the `AnchorView.Props` godoc ("The service compares them and never renders them", `realtime/access.go`), because holders now render it.'
severity: minor
resolution: read_session returns realtime.UntrustedNotice; the tool description calls holders data; the AnchorView.Props godoc is updated.
status: addressed
---
