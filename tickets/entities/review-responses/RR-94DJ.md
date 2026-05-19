---
id: RR-94DJ
type: review-response
title: No startup warning on non-loopback bind + --principal-header
finding: rela-server emits slog.Warn for non-loopback bind. The dangerous combination — non-loopback bind AND --principal-header set — produces no extra log line. Operators scanning startup output would miss the hazard.
severity: significant
resolution: 'Added a second slog.Warn at startup when both conditions hold. Points the operator at docs/security.md explicitly. File: cmd/rela-server/main.go:128-145.'
status: addressed
---
