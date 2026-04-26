---
id: RR-ULGZA
type: review-response
title: 'F4: WithCapturedOutput doc example does not compile'
finding: internal/lua/scripterror.go:144-150 doc comment shows 's.scriptEngine.ExecuteDocument(...).WithCapturedOutput(buf.Bytes())' as the chaining pattern. ExecuteDocument returns error, not *ScriptError, so this won't compile. The actual usage in document.go does an errors.As first. Either fix the example or drop it.
severity: nit
resolution: Dropped the broken chaining example from the WithCapturedOutput doc comment; replaced with a short description of how the data-entry document renderer uses errors.As + this method to attach captured bytes.
status: addressed
---
