---
id: AM-sweep-backlog-conformance
type: automated-measure
title: Every versioning backend captures all changes when more rows are settled than one sweep batch
description: storetest.RunSweepBacklogTests runs for every backend that declares Capabilities.Versioning. It seeds more entities and relations than the SweepNow batch and asserts that never-captured rows drain, that an edit behind a full batch of older captures is captured in one tick, and that an idle tick captures nothing.
kind: test
location: internal/store/storetest/sweepbacklog.go
status: active
---
