---
id: RR-E41T5H
type: review-response
title: Docs runtime compiled worlds per call
finding: docRuntime.worldScope ran worlds.Compile on every assertion (cranky review).
severity: minor
resolution: docRuntime keeps the worlds compiled once in runtime.go and worldScope reads them.
status: addressed
---
