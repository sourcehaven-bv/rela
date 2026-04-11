---
id: RR-CBOWD
type: review-response
title: Context timeout not propagated to Lua script execution
finding: context.WithTimeout creates a timeout but ExecuteFile takes ScriptContext not context.Context. The timeout is effectively theater.
severity: significant
resolution: Removed fake timeout since script.Engine does not support context propagation. Documented limitation. Lua runtime has its own 30s timeout for infinite loops
status: addressed
---
