---
id: RR-LJ4D
type: review-response
title: http.TimeoutHandler buffers responses and breaks SSE
finding: 'The plan''s timeout approach mentions using http.TimeoutHandler on JSON routes only. This is correct in principle but the plan doesn''t specify the mechanism: net/http stdlib''s TimeoutHandler buffers the entire response until the handler returns, which makes Flush() a no-op. Both /api/events and /api/v1/_events use Flusher (watcher.go:212-242), and so does handleCommandExec (commands.go:321) for streaming command output. If any of these is wrapped, it silently breaks. Need explicit per-route exclusion list, not just ''JSON routes''.'
severity: significant
resolution: 'Plan updated: instead of http.TimeoutHandler, use per-handler context deadlines via http.Server''s BaseContext or set ReadTimeout/WriteTimeout to 0 (unlimited) and rely on ReadHeaderTimeout + IdleTimeout + per-mutating-handler context.WithTimeout. SSE handlers and handleCommandExec stream output and explicitly opt out. Documented the streaming-handler list: `/api/events`, `/api/v1/_events`, `/api/command/*`.'
status: addressed
---
