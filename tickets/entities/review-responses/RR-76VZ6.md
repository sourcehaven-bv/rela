---
id: RR-76VZ6
type: review-response
title: Extract startGracefulShutdownCommand helper for reuse
finding: 'If we adopt proc.Cancel/proc.WaitDelay to address critical finding #1, consider extracting startGracefulShutdownCommand(ctx, ...) as a helper for any child process that needs SIGINT+grace semantics (scheduler scripts, scheduled exports, future long-running children). renderWithGraphviz doesn''t need it (dot is fast). Follow-up leverage opportunity, not blocking.'
severity: nit
reason: Premature abstraction for a single call site today. The SSE handler uses the pattern once. Extract when a second call site wants SIGINT+grace semantics, not before.
status: deferred
---
