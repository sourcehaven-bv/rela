---
id: RR-0JT3J
type: review-response
title: 'F5: surface=automation envelope - no path for inline Lua'
finding: 'automation.LuaToExecute has Code (inline) and FilePath fields. For inline automations there is no path, so AC #4''s ''envelope contains script path'' is unsatisfiable as written. Need an explicit identity (e.g., Path = ''automation:<automation-name>'' or a separate AutomationName field). Also need to verify automation name is plumbed to the call site.'
severity: significant
resolution: 'Path = automation.FilePath for file actions; Path = ''automation:'' + automation.Name for inline (Code) actions. AC #4 explicitly states the convention; tests assert it.'
status: addressed
---
