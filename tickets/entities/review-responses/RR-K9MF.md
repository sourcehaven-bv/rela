---
id: RR-K9MF
type: review-response
title: 'Architect/cranky: wsScriptRunner doc rationale is wrong'
finding: wsScriptRunner doc claimed per-call resolution 'preserves correctness under workspace reload' but workspace explicitly does not reload (workspace.go:87-88).
severity: significant
resolution: 'Rewrote doc to explain the real reason: lua.WriteDeps.EntityManager is the only way to thread the active EntityManager into the Lua runtime without entitymanager taking a lua dependency (which would create a cycle).'
status: addressed
---
