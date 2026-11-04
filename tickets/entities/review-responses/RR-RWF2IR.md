---
id: RR-RWF2IR
type: review-response
title: Unknown view 400 is silent for a stale SPA
finding: After a config rename the SPA sends an old view name and swallows the 400.
severity: nit
resolution: 'Planned: unknown view answers empty with slog.Warn; unknown surface stays 400.'
status: addressed
---
