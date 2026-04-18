---
id: RR-RJUCW
type: review-response
title: 'handleOpenFile/handleOpenURL: r.Context() kills fire-and-forget launchers before they dispatch'
finding: 'In internal/dataentry/commands.go:484-514 and :534-552, the handlers do cmd.Start() and return immediately. With exec.CommandContext(r.Context(), ...), r.Context() is cancelled as soon as ServeHTTP returns (per net/http.Request.Context() docs), firing Kill() on the child. On Linux, xdg-open is a shell script that execs a handler (firefox, gedit, nautilus) -- if the handler hasn''t daemonized yet, killing xdg-open kills the handler too. Gedit and Nautilus do NOT daemonize on start. Real bug on Linux; timing-dependent on macOS/Windows. Fix: use a detached context (context.Background() or a separate context.WithTimeout) for fire-and-forget launchers, then the process lives beyond the handler return.'
severity: significant
resolution: Extracted the OS-specific launcher-command builders into openFileCommand and openURLCommand helpers in internal/dataentry/commands.go. The helpers use exec.Command (no context) so launcher processes survive the HTTP handler's return; added go func(){ _ = cmd.Wait() }() after Start() to reap the zombie. Fixes the Linux xdg-open/gedit/nautilus bug. Also applied //nolint:noctx comments on the unbound exec.Command calls with explanatory 'fire-and-forget launcher' comments.
status: addressed
---
