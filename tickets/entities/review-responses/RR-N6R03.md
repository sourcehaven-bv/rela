---
id: RR-N6R03
type: review-response
title: SSE script cancellation now SIGKILLs instead of SIGINT-with-grace on client disconnect
finding: 'In internal/dataentry/commands.go:356, wrapping the SSE script runner with exec.CommandContext(r.Context(), ...) makes exec.CommandContext default cmd.Cancel to Process.Kill() (SIGKILL). But handleCommandCancel at line 443 deliberately sends SIGINT first and SIGKILLs only after a 3s grace period. With the new wiring, any client disconnect (closed tab, flaky wifi, browser background throttling) triggers immediate SIGKILL, bypassing the graceful-shutdown protocol. A script that catches SIGINT to flush output or commit a transaction no longer gets the chance. Fix: set proc.Cancel to send syscall.SIGINT and proc.WaitDelay = 3*time.Second so context cancellation honors the same contract as explicit cancel.'
severity: critical
resolution: Set proc.Cancel to syscall.SIGINT and proc.WaitDelay = cancelGrace (3s) in handleCommandExec (internal/dataentry/commands.go:362-365). Extracted cancelGrace as a shared const used by both handleCommandCancel's explicit-cancel path and the context-triggered cancel. Client disconnect now fires SIGINT+3s grace+SIGKILL, matching explicit cancel semantics. Scripts that catch SIGINT to flush state get the same chance either way.
status: addressed
---
