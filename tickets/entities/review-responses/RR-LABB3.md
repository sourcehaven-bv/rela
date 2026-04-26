---
id: RR-LABB3
type: review-response
title: 'F3: *bytes.Buffer plumbing requires more signature changes than plan lists'
finding: 'Plan says ''pass *bytes.Buffer to caller alongside the error'' for document.go:244-248 only. But for actions, the buffer is constructed in script/action.go:59 and never returned, so Engine.ExecuteAction''s signature must change. Same for automations: ScriptExecutor in workspace.go and script.Engine.ExecuteCode/ExecuteFile don''t expose stdout. The ''Files to modify'' list undercounts: internal/script/action.go, internal/script/executor.go, and the ScriptExecutor interface in workspace.go all need touching. Effort under-estimated.'
severity: critical
resolution: Added internal/lua/runtime.RunFileWithCapture / RunStringWithCapture helpers so capture pairs with the run, avoiding *bytes.Buffer leakage through every Engine signature. Files-to-modify list expanded to include internal/script/action.go, internal/script/executor.go. Automations explicitly skip captured output (decision documented).
status: addressed
---
