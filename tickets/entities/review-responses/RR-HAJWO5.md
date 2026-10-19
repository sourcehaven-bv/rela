---
id: RR-HAJWO5
type: review-response
title: The watch loop takes the whole channel service
finding: "ChannelWatchCmd.watch (internal/cli/channel_watch.go:352) takes channelService, which includes Post, Edit, Delete and Create, but it uses only Read and Subscribe. Design review section 13 says \"the watch path holds only the reader\". channelService (internal/cli/channel.go:62-67) recreates the 8-method bundle that the review split up.\tA later edit to the watch loop can call a write method, and no type stops it. Test fakes for watch must implement all eight methods.\tDeclare `watch(ctx, svc interface{ channelReader; channelWaker }, tick)` and give each Run the narrowest interface it uses. Keep the composite only on the binding struct."
severity: minor
resolution: watch takes watchService (channelReader + channelWaker). The composite stays only on the binding struct.
reason: watch takes watchService (channelReader + channelWaker). The composite stays only on the binding struct.
status: addressed
---
