---
id: RR-4M3Q2T
type: review-response
title: rollupSource.add drops a construction error silently
finding: A NewRollups error made every roll-up vanish without a log line.
severity: minor
resolution: Logged with slog.WarnContext.
status: addressed
---
