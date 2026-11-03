---
id: RR-M4DXZD
type: review-response
title: Interfaces grow past the three-method rule; a separate typist capability is cleaner
finding: 'CLAUDE.md asks for consumer-side interfaces of one to three methods. Step 13 adds Typing to dataentry messageWriter (3 methods now, internal/dataentry/channels_handler.go:47-51), and step 15 adds it to mcp.ChannelWriter (3 now, internal/mcp/tools_channels.go:29-33) and to unavailableChannels. Plan change: declare channelTypist { Typing(ctx, addr) (realtime.TypingResult, error) } in dataentry and ChannelTypist in mcp. ChannelDeps gets a Typist field that is nil under stdio, so tool registration keys on Typist != nil and not on !RefuseSignals. This makes "absent under stdio" structural.'
severity: minor
resolution: 'Plan updated: dataentry gets its own channelTypist interface (one method); MCP gets no send tool and so no new interface or ChannelWriter change.'
status: addressed
---
