---
id: RR-1ZNL1
type: review-response
title: 'F4: effects.Errors []string change ripples wider than ''one test reader'''
finding: 'Plan claims ''only workspace_test.go:1593 reads it''. Actually: workspace.go:817 and 868 do result.AutomationErrors = append(..., effects.Errors...) - i.e., this is a public-API field. UpdateResult.AutomationErrors []string is read by workspace/manager.go:48,83 and asserted by ~13 test sites. Also: most automation errors are NOT Lua errors (validation failures, write errors) - so the type must be []error with *ScriptError values for Lua failures, not []ScriptError. AC #4''s ''or'' phrasing leaves real ambiguity.'
severity: significant
resolution: 'AC #4 rewritten: type is []error (Lua failures = *lua.ScriptError, non-Lua remain plain errors). Files list now includes manager.go (UpdateResult.AutomationErrors) and acknowledges ~13 test sites. Risk row updated to high likelihood with separate-commit mitigation.'
status: addressed
---
