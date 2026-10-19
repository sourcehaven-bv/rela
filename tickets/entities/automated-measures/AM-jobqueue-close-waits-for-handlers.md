---
id: AM-jobqueue-close-waits-for-handlers
type: automated-measure
title: Job queue Close waits for running handlers; tests restore the whole default logger
description: The jobstest conformance case CloseWaitsForRunningHandlers fails when a backend's Close returns while a handler still runs. TestCapture_RestoresLogOutput fails when logtest leaves the log package output on a finished capture. A forbidigo rule bans slog.SetDefault in tests outside logtest.
kind: test
location: internal/jobs/jobstest/jobstest.go (CloseWaitsForRunningHandlers); internal/testutil/logtest/logtest_test.go (TestCapture_RestoresLogOutput); .golangci.yml (forbidigo)
status: active
---
