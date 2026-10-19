---
id: RR-FGDN18
type: review-response
title: Condition evaluation errors are swallowed with no log
finding: internal/realtime/authz.go:292-309 maps a SetVar, Bind or Eval failure to "does not hold", which is correct fail-closed behaviour. But the operator only sees the `denied:` text and has nothing to debug a condition that never holds. Log the error once (slog.Warn with the rule key) before returning false.
severity: minor
resolution: conditionHolds now takes the rule key. A SetVar, Bind or Eval failure logs slog.WarnContext with the rule, the anchor id and the error, then refuses as before.
status: addressed
---
