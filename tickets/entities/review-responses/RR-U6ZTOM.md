---
id: RR-U6ZTOM
type: review-response
title: Plan's SandboxErr() call would nil-panic on the nil-runner path it is meant to warn about
finding: 'internal/dataentry/app.go:1033-1043 assigns app.attachmentRunner ONLY when attachment.NewCmdRunner succeeds; on error it logs slog.Warn("command runner unavailable") and leaves the field nil, after which handlers_attachment.go:300-302 constructs the PolicyProcessor with a nil runner (MIME validation only). The plan''s approach — "the runner reports no usable sandbox" via runner.SandboxErr() — dereferences a possibly-nil *attachment.CmdRunner, panicking at startup in one of the two cases the warning exists to cover. The WARN condition must be a disjunction over both degraded states: (runner == nil) OR (runner.SandboxErr() != nil). Note these are genuinely different failures — constructor failure vs. an unusable host sandbox — and the message should not claim the systemd cause for the former.'
severity: significant
resolution: 'Plan updated: the WARN condition is now the explicit disjunction `scanConfigured AND (runner == nil OR runner.SandboxErr() != nil)`, with a note that the two causes are distinct (constructor failure vs unusable host sandbox) and the message must not attribute the systemd cause to a nil runner. Added an edge case asserting the nil-runner path warns without panicking.'
status: addressed
---
