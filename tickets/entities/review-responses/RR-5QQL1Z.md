---
id: RR-5QQL1Z
type: review-response
title: Unrestricted(nil) returned a typed nil, converting lua's documented DENY into a nil-deref panic
finding: 'The godoc claimed ''Returns nil for a nil store, which the Lua bindings treat as a DENY (RR-X9NVHI)''. That is the typed-nil-in-interface trap and the claim was false. Verified empirically with a probe against the real packages: assigning a nil *UnrestrictedReader into lua.ReadDeps.VisibleReader yields a NON-nil interface, so lua''s `VisibleReader == nil` guard is skipped and GetEntity nil-derefs. Probe output: ''VisibleReader == nil ? false'' then ''PANIC CONFIRMED: invalid memory address or nil pointer dereference''. No script path recovers, so this is a process-killing panic at request time instead of the clean ''no reader is configured'' Lua error. Not reachable today (appbuild and dataentry both nil-check the store first) but the godoc advertised a guarantee future wiring would rely on.'
severity: critical
resolution: Unrestricted now PANICS on a nil store, per CLAUDE.md 'constructors reject nil required fields'. A nil store is a wiring bug; failing loudly at construction, in the stack of the offending code, beats both a silent typed-nil and a request-time panic. Godoc rewritten to explain the trap rather than assert the false guarantee.
status: addressed
---
