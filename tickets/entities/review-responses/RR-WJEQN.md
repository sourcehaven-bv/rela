---
id: RR-WJEQN
type: review-response
title: 'F9: MCP envelope as JSON-in-codeblock breaks IsError flag'
finding: 'Existing MCP tools use mcp.NewToolResultError(err.Error()) which sets IsError: true. Plan switches lua_run/lua_eval to NewToolResultText with JSON-in-codeblock - clients keying off IsError to differentiate failures will see lua failures as successful. Use NewToolResultError with the JSON string instead.'
severity: minor
resolution: 'Plan now uses mcp.NewToolResultError(jsonString) for both lua_run and lua_eval, preserving IsError flag. AC #3 asserts both envelope JSON and IsError=true.'
status: addressed
---
