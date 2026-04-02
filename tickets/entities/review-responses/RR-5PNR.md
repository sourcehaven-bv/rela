---
id: RR-5PNR
type: review-response
title: Duplicated scriptsDir constant
finding: The scriptsDir = "scripts" constant is duplicated in workspace.go and mcp/tools_lua.go. Divergence risk if one changes without the other. Extract to a shared location.
severity: minor
reason: The scriptsDir constant is used in two separate packages (workspace and mcp) with different responsibilities. Moving it to a shared location would create unnecessary coupling. The duplication is minimal (one constant) and the value is unlikely to change.
status: wont-fix
---
