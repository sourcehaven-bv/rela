---
id: RR-P8AU
type: review-response
title: No execution timeout for Lua validation scripts
finding: 'The `luaExecutor.validate()` method at `/Users/jeroen/Work/sourcehaven/rela-3/internal/validation/lua.go:64` creates a Lua runtime but does NOT apply a timeout. The main `lua.Runtime` has a `DefaultTimeout` of 30 seconds and `applyTimeout()` is called in `RunFile()` and `RunString()`, but the validation code uses `LoadString` + `PCall` directly without setting any timeout. A malicious or buggy validation script with an infinite loop (`while true do end`) will hang forever, blocking the entire validation run. This is a denial-of-service vulnerability. FIX: Either (1) call `runtime.RunString(code)` instead of manual LoadString+PCall and check return value, OR (2) manually apply timeout via `runtime.LState().SetContext(ctx)` before PCall.'
severity: critical
resolution: Added 5-second timeout via context.WithTimeout before PCall. Added TestLuaValidation_Timeout to verify timeout handling.
status: addressed
---
