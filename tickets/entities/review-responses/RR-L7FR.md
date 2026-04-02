---
id: RR-L7FR
type: review-response
title: No timeout on Lua script execution
finding: The Lua runtime has no execution timeout. A malicious or buggy script could run forever, causing denial of service in the MCP server, hung automation processing, and resource exhaustion. Use a context with timeout or gopher-lua's LState.SetContext() with deadline.
severity: significant
resolution: Added 30-second execution timeout using context.WithTimeout and ls.SetContext(ctx). This prevents infinite loops or resource exhaustion from runaway Lua scripts. The timeout constant luaExecutionTimeout is defined at package level for easy configuration.
status: addressed
---
