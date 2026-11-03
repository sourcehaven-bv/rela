---
id: RR-Y9CD2X
type: review-response
title: Stdio schema reload with an unusable acl.yaml leaves MCP serving a retired assembly
finding: reload() sets s.svc=next and builds deps (mcp_wiring.go:168), then old.CloseAssembly() runs (:177). Only after that does srv.ReloadDeps run (:211), and it refuses the bundle when channelDeps() returned empty because PolicyChannelService failed (:340-348, server.go:329). The server keeps the old deps, whose job queue, mail and sweeps are stopped. A schema edit that removes a type acl.yaml names is enough to trigger it. Validate the deps before publishing and retiring, or have channelDeps return a refusing ChannelReader instead of nothing.
severity: critical
resolution: reload now takes the publish function (srv.ReloadDeps) and calls it under mu with the new bundle before it swaps s.svc. If the server rejects the bundle, the new assembly is retired and the old one stays current and serving. Only after a successful publish is the old assembly retired. Also, an acl.yaml that cannot be built now gives mcp.UnavailableChannels instead of empty deps, so ReloadDeps never rejects the bundle for that reason. Tested by TestMCPServices_ReloadRejectedKeepsPrevious and TestMCPStdio_BrokenACLOnReload.
status: addressed
---
