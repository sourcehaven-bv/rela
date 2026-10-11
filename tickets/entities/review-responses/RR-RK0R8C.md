---
id: RR-RK0R8C
type: review-response
title: World grant stays stale for a remote watch
finding: 'Security review: Rebind replaced only the acl.Request. World selection on remote MCP checks readGateFromContext, the gate attachACLRequest built around the call''s first Request, so a world grant revoked mid-watch kept being honoured for up to 50 s.'
severity: significant
resolution: mcpSelectWorld and mcpWorldReadable now build the gate from the acl.Request on ctx (mcpWorldGate), which is the fresh one after Rebind. TestMCPWorldCheck_UsesRequestOnContext pins both directions, with a mutation check.
status: addressed
---
