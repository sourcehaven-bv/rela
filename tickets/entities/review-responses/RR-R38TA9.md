---
id: RR-R38TA9
type: review-response
title: DenyTracer godoc cited FindOrphans as the diagnostic surface, but no script can call it
finding: 'DenyTracer''s godoc reassured that FindOrphans ''CAN report failure rather than looking like an empty graph''. Verified against internal/lua/runtime.go:697-699: the only script-bound traversals are trace_from, trace_to and find_path — all three silent-nil. FindOrphans is unreachable from Lua. So the scripting surface is 100% silent, and luaTraceFrom maps nil to LNil, the identical value returned for a nonexistent ID: a script cannot distinguish ''gate broken, refusing'' from ''no such entity''.'
severity: significant
resolution: Added an explicit CAVEAT paragraph to DenyTracer stating that refusal is invisible to scripts, that the three Lua-bound traversals are exactly the three that cannot report failure, and that the wiring-site slog.Error is the only signal (correlate by timestamp). Documented the tradeoff as deliberate — seeing less than the truth is the safe direction — and noted that surfacing it properly requires an error-returning traversal on tracer.Tracer, an interface change tracked separately.
status: addressed
---
