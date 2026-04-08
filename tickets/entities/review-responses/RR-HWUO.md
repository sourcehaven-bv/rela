---
id: RR-HWUO
type: review-response
title: 'F13: jsonUnmarshal wrapper is pointless indirection with a false rationale'
finding: redact.go contained a jsonUnmarshal wrapper around json.Unmarshal with a comment claiming it 'exists so errors.go can call into encoding/json without an import cycle'. Files in the same Go package cannot have an import cycle with each other; the wrapper was zero-value indirection.
severity: nit
resolution: Wrapper deleted from redact.go. errors.go now imports encoding/json directly and calls json.Unmarshal at the call site. redact.go no longer needs the json import.
status: addressed
---
