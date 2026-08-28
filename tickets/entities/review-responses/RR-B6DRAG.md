---
id: RR-B6DRAG
type: review-response
title: Field-axis ceiling failed open when ctx carried no acl.Request
finding: 'applyClientCeiling (internal/affordances/resolver.go:686-690) returned early when acl.FromContext(ctx) was nil, so a principal stamped WITHOUT an acl.Request kept its acting user''s full field visibility — the ceiling silently did not apply. Twenty lines below in the same file, resolveViaDeclarative hits the identical condition and opens a fresh Request instead. So within a single FieldVerdicts call, on a ctx carrying a principal but no Request: role resolution worked, the client ceiling did not. Every test in internal/affordances/ceiling_test.go bound via the ctxFor helper, so the whole suite passed over the hole. Not reachable in production (attachACLRequest covers /api/, ScriptReader.bind covers Lua), but visibility.PolicyRedactor.HiddenProperties forwards whatever ctx it is handed and binds nothing itself, so the guarantee rested on wiring rather than structure. This is the fail-open direction, in the axis carrying the ticket''s originating use case (MCP-as-HR-user must not see person.salary).'
severity: critical
resolution: 'applyClientCeiling now opens its own Request when ctx has none, mirroring resolveViaDeclarative. An unstamped principal still returns early — it has no verified principal_type, so no baseline can match and a ceiling can only narrow. Pinned by TestCeiling_AppliesWithoutAnACLRequestOnContext (reproduced the fail-open before the fix) and TestCeiling_UnstampedPrincipalIsUnrestricted (the counterpart: recovery must not invent a ceiling).'
status: addressed
---
