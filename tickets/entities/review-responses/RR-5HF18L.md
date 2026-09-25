---
id: RR-5HF18L
type: review-response
title: A late success after a reap is discarded, so the task reruns
finding: A script that ignores ctx can succeed after its run was abandoned. FinishRun returns Applied=false, the ladder stays failing, and the task runs again 5 minutes later.
severity: minor
resolution: FinishRun on an ABANDONED run with a success now applies the outcome to the task (LastRun = run CreatedAt, ladder cleared) and reports Finished.LateSuccess; the run keeps its abandoned status. A late failure is still discarded. Logged as 'late success recorded'. Pinned by conformance LateSuccessAfterReapStampsLastRun and LateFailureAfterReapIsDiscarded on both backends.
status: addressed
---
