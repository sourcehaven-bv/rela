---
id: FEAT-2M4D
type: feature
title: Stress / soak harness for rela-server diagnostics
summary: Browser-driven concurrent workload generator with pprof-on-breach, used to reproduce intermittent server hangs
description: frontend/stress/ contains a Playwright-based stress runner. Boots a fresh rela-server against an isolated /tmp project copy, drives N concurrent Chromium or Firefox BrowserContexts through parameterised scenarios (currently watcher-pressure). Includes a schema canary that bypasses the browser, periodic progress checkpoints, and automatic pprof goroutine/heap/mutex/block capture on invariant breach. Designed for use during root-cause investigation of intermittent issues like BUG-FMS1, where a single HAR is not enough to diagnose a hang.
priority: medium
status: implemented
---
