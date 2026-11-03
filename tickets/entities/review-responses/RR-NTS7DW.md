---
id: RR-NTS7DW
type: review-response
title: Stdio channel tools keep the acl.yaml from startup until restart or schema reload
finding: '`mcpServices.channelDeps` (internal/cli/mcp_wiring.go:340) reads acl.yaml only when `deps()` runs, and only schema.yaml and realtime.yaml are watched (lines 203, 238), so a narrowed acl.yaml does not limit the agent until restart, contrary to the docs ("applies acl.yaml", docs/mcp-server.md:531). If acl.yaml is broken at the next schema reload, `validateFor` (internal/mcp/server.go:329) rejects the whole reload and the old policy stays. This is fail-closed toward the previous policy and not a boundary locally, but the doc should say "as of startup" or acl.yaml should be watched.'
severity: minor
resolution: 'Same fix as the acl.yaml watch finding: stdio watches acl.yaml and republishes the bundle on each change. Tested by TestMCPStdio_ACLWatch.'
status: addressed
---
