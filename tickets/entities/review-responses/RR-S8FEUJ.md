---
id: RR-S8FEUJ
type: review-response
title: Concurrency semaphore is load-bearing, not a backstop; bound must be specified
finding: The DecodeConfig pixel cap bounds ONE decode's allocation, but N concurrent uploads each near the cap (50MP×4B ≈ 200MB) still OOM the process. The plan listed the semaphore only as a 'backstop'. GOMEMLIMIT is a soft GC target and won't prevent the transient spike.
severity: significant
resolution: Reclassified the concurrency semaphore as a REQUIRED control with a specified default bound (e.g. runtime.GOMAXPROCS-based, small). imgproc owns a package-level weighted semaphore acquired around each Normalize; MaxPixels and the semaphore bound are chosen together so worst-case = bound × (MaxPixels×4B) stays within a documented memory budget. Added as an explicit AC + a test asserting the semaphore caps in-flight decodes.
status: addressed
---
