---
id: RR-KTTHOJ
type: review-response
title: Open has no nil contract for log
finding: Open(ctx, nil) would panic on the first warning.
severity: nit
resolution: A nil log falls back to slog.Default(); the godoc says so and TestOpen_NilLogFallsBackToDefault pins it.
status: addressed
---
