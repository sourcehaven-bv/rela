---
id: RR-VHA467
type: review-response
title: Re-run convergence after a failed row move was untested
finding: moveThreads' documented ordering (thread waits at the destination until a re-run moves the row) had no test.
severity: significant
resolution: TestMigrateFace_RerunConvergesAfterTheRowMoveFails fails the first row Tx, asserts the thread waits at the destination and the row stays, then re-runs and asserts both converge.
status: addressed
---
