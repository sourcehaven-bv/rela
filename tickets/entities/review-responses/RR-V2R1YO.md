---
id: RR-V2R1YO
type: review-response
title: Duplicate re-export of lifecycle names in realtime
finding: internal/realtime/lifecycle.go:17-21 aliases realtimecfg.LifecycleDelete and wraps LifecycleOpNames in IsLifecycleOp, which allocates a slice per route parse. Fine as a boundary choice, but say why in the godoc (dataentry should not import realtimecfg), or let dataentry use realtimecfg directly.
severity: nit
resolution: Kept the boundary re-export because dataentry may not import realtimecfg under .go-arch-lint.yml, and the godoc now says so. IsLifecycleOp is a switch that allocates nothing. TestIsLifecycleOp pins that it matches LifecycleOpNames.
status: addressed
---
