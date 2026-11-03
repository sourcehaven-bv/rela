---
id: RR-IGDX46
type: review-response
title: ChannelReader grows past the consumer-interface size rule
finding: '`mcp.ChannelReader` (`mcp/tools_channels.go:20-25`) already has 3 methods. Adding `Session` makes 4, and CLAUDE.md asks for 1-3 at the call site. Change: declare a separate `SessionReader interface { Session(ctx, realtime.RawAddress) (realtime.SessionRead, error) }` in `tools_channels.go`. Add it to `ChannelDeps`, with the same nil rules, and to the `unavailableChannels` stub.'
severity: minor
resolution: Added a separate SessionReader consumer interface in tools_channels.go, wired in ChannelDeps and the unavailableChannels stub; ChannelReader stays at 3 methods.
status: addressed
---
