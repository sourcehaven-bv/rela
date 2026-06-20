---
id: RR-H5A0MZ
type: review-response
title: 'default/u: branch fails silently — no guard against an unhandled type reaching production'
finding: 'The default branch (canonical.go:177) is, by the package''s own logic, the danger zone — anything reaching it is fmt.Sprintf''d and almost certainly diverges across backends (proven by the time.Time and map[any]any criticals). Yet it fails SILENTLY (emits u:... and moves on). For load-bearing sync code, hitting default should be observable. FIX: add a test asserting the full set of types each backend can actually yield (time.Time, map[any]any, uint64, json.Number if it leaks) is explicitly handled, so a future yaml/json/pgx upgrade introducing a new type is caught rather than silently corrupting sync. Relatedly (L2): normalize value types at one boundary instead of teaching the hasher every decoder type (whack-a-mole).'
severity: significant
resolution: The hasher's writeValue now PANICS on any type other than the closed normalized set (nil/string/bool/int64/float64/[]any/map[string]any) — a normalization bug fails loudly instead of silently hashing an unverified value. normalize() owns the (now narrow) fallback for genuinely-unknown decoder types (fmt.Sprintf), but every type either backend actually produces is explicitly folded above it. Combined with the cross-backend fuzz, a future yaml/json/pgx upgrade introducing a new type would surface as a fuzz failure or panic, not silent corruption.
status: addressed
---
