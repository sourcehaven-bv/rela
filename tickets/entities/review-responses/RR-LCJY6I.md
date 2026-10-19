---
id: RR-LCJY6I
type: review-response
title: Weak assertions in wiring tests
finding: TestMCPStdio_RealtimeReload asserts that a broken edit did not apply after only a 500ms sleep, so it passes if the watcher is just slow. TestRemoteMCPDeps_NoChannelsWithoutRealtime (remote_mcp_channels_test.go:107) has conditions that are nearly always false, so it asserts little.
severity: nit
resolution: Replaced the 500ms sleep in TestMCPStdio_RealtimeReload with a direct reloadRealtime call; the watcher path is still pinned by the earlier Eventually. TestRemoteMCPDeps_NoChannelsWithoutRealtime now requires a non-nil service that is disabled and wired as Reader, and checks over a real client that tools/list has no channel or message tool. The Channels test checks that read_channel is listed.
status: addressed
---
