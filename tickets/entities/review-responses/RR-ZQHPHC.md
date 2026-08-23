---
id: RR-ZQHPHC
type: review-response
title: provision.go uses slog.Warn instead of slog.WarnContext
finding: The new guard in provision.go logs via slog.Warn while the two other new call sites (router.go, jwtgate.go) use the WarnContext variant. ctx is in scope right there. Inconsistent with the surrounding new code and drops trace correlation.
severity: nit
resolution: Changed to slog.WarnContext(ctx, ...) in provision.go, matching the two other new call sites and restoring trace correlation. ctx was already in scope.
status: addressed
---
