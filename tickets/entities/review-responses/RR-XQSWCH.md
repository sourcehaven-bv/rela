---
id: RR-XQSWCH
type: review-response
title: The moved startup warning is no longer tested at its call site
finding: "warnUnassignedUser moved from newMCPServices to McpCmd.Run (internal/cli/mcp.go:39), which is coverage-ignored. TestMCPStdio_WarnsUnassignedUser (internal/cli/mcp_wiring_realtime_test.go:323) now calls the helper directly with tc.user, so the new \"run-as unassigned\" case tests only the helper.\tSomeone deletes or reorders the call in McpCmd.Run, and no test fails. A run-as agent with no acl.yaml assignment then gets \"entity not found\" from the channel tools with no startup hint.\tHave McpCmd.Run call a testable function, for example startMCP(identity, ...), that performs the warning, and drive the test through it with a --run-as identity. Or pass the identity into newMCPServices so the existing test seam covers the call."
severity: minor
resolution: McpCmd.Run calls openMCPServices(startDir, identity), which opens the project and warns. TestMCPStdio_WarnsUnassignedUser drives it with a real identity, including a --run-as identity.
reason: McpCmd.Run calls openMCPServices(startDir, identity), which opens the project and warns. TestMCPStdio_WarnsUnassignedUser drives it with a real identity, including a --run-as identity.
status: addressed
---
