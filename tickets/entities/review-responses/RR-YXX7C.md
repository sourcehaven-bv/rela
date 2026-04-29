---
id: RR-YXX7C
type: review-response
title: Inert vi.useFakeTimers() in success-path test
finding: The all-success test enables fake timers but never advances them. setTimeout(..., 350) is scheduled but its effects (invalidateAll, onComplete) are not asserted. Either drop the fake timer or actually exercise the timer with vi.advanceTimersByTime(350).
severity: nit
resolution: Removed the inert vi.useFakeTimers() call from the all-success test and the now-dead afterEach(vi.useRealTimers). The test asserts the dialog is not opened and the success toast fires; both happen synchronously after Promise.allSettled, before the 350ms setTimeout. The setTimeout-scheduled invalidateAll/onComplete are not the contract under test here.
status: addressed
---
