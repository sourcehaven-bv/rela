---
id: RR-XC2RYOW
type: review-response
title: Definition load per row
finding: Each finished run loaded and compiled its flow version.
severity: significant
resolution: Versions come from the engine's FlowCache, so the cost is one compile per distinct version, and only on the detail paths (Run and SubjectRuns).
status: addressed
---
