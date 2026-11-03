---
id: RR-UTM1MF
type: review-response
title: Unbounded wait for the handler to start in CloseGivesUpOnAStuckHandler
finding: <-started has no timeout, so a job that never starts hangs until the go test timeout.
severity: minor
resolution: Uses a select with a 10s timeout, as jobstest does.
status: addressed
---
