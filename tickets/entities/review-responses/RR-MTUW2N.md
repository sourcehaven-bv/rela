---
id: RR-MTUW2N
type: review-response
title: PermitsRead mid-stream error must fail-closed (drop frame, keep connection); can't send HTTP status after headers flushed
finding: 'PermitsRead returns (bool, error); other consumers map errors via writeGateError to 504/500/cancel-silent, but SSE has already flushed headers + '': connected'' (watcher.go:331) so no status can be sent. Required behavior: on error, DROP the frame (never write — writing on error is fail-open, the exact leak). Do NOT close the connection per-error (triggers reconnect storm against a flaky backend); drop the single frame, keep the stream, the client''s eventual refresh/reconnect re-snapshot reconciles. Distinguish context.Canceled (client hung up) from real backend error to avoid log noise. Pin with an injected MatchingIDs error asserting no frame written + connection stays up.'
severity: significant
resolution: 'Carried into the final design as AC7: a ReadQuery(type) error in the per-connection handleSSE loop drops the `{type}` nudge and keeps the connection (headers already flushed — no status sendable; writing anyway is fail-open). Distinguish context.Canceled (client hung up → return) from a real error (→ drop + Warn + continue), reusing the filterVisibleIncludes fail-closed pattern. Pinned by an injected-ReadQuery-error test.'
status: addressed
---
