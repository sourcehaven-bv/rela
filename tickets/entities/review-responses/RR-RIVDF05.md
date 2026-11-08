---
id: RR-RIVDF05
type: review-response
title: Empty host config gives the wrong AI/HTTP error and the HTTP client is a package global
finding: lua/context.go:66-73 adds no provider without ai.yaml; http global needs Capabilities.HTTP; lua/http.go:104 package var.
severity: minor
resolution: Per-runtime lua.WithHTTPClient option with a refusing transport and a refusing AI provider; AC5 uses an automation declaring HTTP and AI capabilities.
status: addressed
---
