---
id: RR-W7AE8W
type: review-response
title: Zero-value MCPComments panics
finding: MCPComments is exported and a zero value has a nil handler
severity: nit
reason: The handler field is unexported, so outside the package only the App's host can build a usable value; the one zero value (a pointer-identity test in cmd/rela-server) is never called. A nil guard would hide a wiring bug.
status: wont-fix
---
