---
id: RR-0WDY5Y
type: review-response
title: Data race on Outbox.cancel between Start and Stop; Stop could see nil and leak the worker
finding: 'Start wrote o.cancel inside startOnce while Stop read it inside stopOnce. Two different sync.Once values establish no happens-before with each other, so this is a genuine data race — confirmed by reproducing it under -race. Worse than the detector failure: Stop could observe a nil cancel while a worker was running, take the ''never started'' branch, and return without cancelling, leaking the goroutine for the process lifetime. The single current call site happens to order Start before Stop, which is exactly why this would have surfaced later when mail was wired differently.'
severity: critical
resolution: The context is now created in NewOutbox, so Stop never reads a field Start writes. A started atomic.Bool distinguishes 'no worker to wait for' from 'worker running', and cancel is called unconditionally so a Start racing a Stop yields an immediately-cancelled worker rather than a leak. Pinned by TestOutbox_StartStopRace (50 iterations of the exact interleaving, meaningful under -race) and TestOutbox_StopBeforeStartPreventsWork.
status: addressed
---
