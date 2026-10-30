---
id: RR-PV5LDV
type: review-response
title: Only the HTTP handler test pins the derived rule; MCP and CLI show it too
finding: 'DenialError.Rule reaches MCP ("(rule: ...)", internal/mcp/tools_session_test.go:279-280) and the CLI (internal/cli/channel_test.go:435). The plan tests the derived rule only in channels_handler_test.go. All three render the same field, so this is optional. Adding one MCP case would pin the text an agent sees.'
severity: nit
reason: MCP and the CLI render the same DenialError.Rule field that the handler test and TestPost_DefaultIn already pin, so a third case adds no coverage of new code. The plan states this under Integration.
status: wont-fix
---
