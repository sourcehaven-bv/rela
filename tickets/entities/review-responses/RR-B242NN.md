---
id: RR-B242NN
type: review-response
title: acl.yaml edits are not picked up by stdio channel tools until restart or schema save
finding: Stdio subscribes only to schema.yaml and realtime.yaml (mcp_wiring.go watchSchema/watchRealtime). The gating policy is frozen at build time, and the policy that realtimecfg uses for its leak checks is loaded separately (policyOnce in realtime.go:163), so the two can differ. Either watch acl.yaml or document this in docs/mcp-server.md and docs/realtime.md.
severity: minor
resolution: Stdio now watches acl.yaml (watchACL). Each change rebuilds the bundle through republish and calls srv.ReloadDeps; the watch is stopped in Close. Tested by TestMCPStdio_ACLWatch, which grants a role and then breaks the file.
status: addressed
---
