---
id: RR-OQBYQS9
type: review-response
title: A run write inside a store Tx would wait for its own lock
finding: txFlowRuns opens a store Tx for Create and Update. Called from inside a store Tx it would block on the write lock its own caller holds.
severity: minor
resolution: Create and Update return errFlowRunsNested on a ctx marked as in a Tx.
status: addressed
---
