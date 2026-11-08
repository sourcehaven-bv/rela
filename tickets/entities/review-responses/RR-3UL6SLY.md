---
id: RR-3UL6SLY
type: review-response
title: Nil or empty seam option overrides the safe default
finding: WithMailTransport(nil), WithHTTPClient(nil), WithAIProvider(nil) and WithOutputDir("") assigned unconditionally and replaced NewInMemory's safe defaults, reopening real network and project-root writes.
severity: significant
resolution: Each option now ignores a nil or empty value; TestNewInMemory_ZeroOptionsKeepTheDefaults pins it.
status: addressed
---
