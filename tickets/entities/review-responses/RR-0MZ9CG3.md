---
id: RR-0MZ9CG3
type: review-response
title: Cancel uses the current flow's permissions
finding: Cancel took the manual start permissions from the current flows.yaml, not from the version the run follows.
severity: minor
resolution: Cancel now uses the run's own version and falls back to the current flow only when that version no longer compiles.
status: addressed
---
