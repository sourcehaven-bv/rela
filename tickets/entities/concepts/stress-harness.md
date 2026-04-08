---
id: stress-harness
type: concept
title: Stress / soak harness
summary: Browser-driven concurrent workload generator for diagnosing rela-server hangs and intermittent issues
description: Located at frontend/stress/. Spins up a fresh rela-server against an isolated /tmp project copy, drives N parallel Playwright BrowserContexts (Chromium or Firefox) through scenarios that mix reads, edits, and file-watcher pressure. Includes a schema canary that bypasses the browser to measure server-side latency independently. On invariant breach (5xx, latency, console errors, slow op) it captures pprof goroutine/heap/mutex/block profiles for post-mortem. The harness exists because intermittent server hangs cannot be diagnosed from HARs alone — we need server-side instrumentation and a reproducible workload.
package: frontend/stress
layer: infra
status: draft
---
