---
id: RR-3JRSFV
type: review-response
title: World names admit spaces and slashes; tightening is free now and breaking later
finding: 'metamodel loader.go validates world names via ValidateSchemaName, which only rejects empty names, quotes/backslashes/control chars, and surrounding whitespace. So `world: "a b"` and `world: "world/with/slash"` both load. The comment at that very line says ''A world name reaches URLs, acl.yaml and CLI flags in later steps'', so PR-B/C/D will need percent-encoding or a tightening migration. Not a PR-A bug — nothing consumes the name yet — but tightening now is free and tightening later is a breaking change for anyone who already declared such a name.'
severity: nit
reason: 'Deferred to the step that first consumes a world name in a URL or flag, rather than fixed blind in PR-A. Two reasons. (1) Binding architect decision Q10: Step 2 exposes NO ?world= / --world parameter at all — request-level selection lands in Step 3 (TKT-DN37J2) together with its grant check. So no world name reaches a URL or a CLI flag anywhere in this arc, and the correct charset is a function of the surface that will carry it, which is not yet designed. Choosing a restriction now means guessing at that surface. (2) The dangerous case is already closed: PR-A rejects the EMPTY world name (the fail-open one — it would make a lookup with an unpopulated name return a real non-default world), and rejects the reserved name `default` case-folded. What remains admits only awkward names, not unsafe ones, and an unknown name fails closed at Compiled.Lookup. Recorded as owed work on TKT-DN37J2 alongside the other Step-3 items, so the tightening lands with the surface that motivates it. Raised as a nit by the reviewer, who also judged it not a PR-A bug.'
status: deferred
---
