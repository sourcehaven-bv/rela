---
id: RR-VEACKR
type: review-response
title: 'Verified: the storetest CAS suite genuinely catches a backend that degrades to an unconditional write'
finding: 'Recording an audit result rather than a defect, because the value of this whole ticket rests on the conformance suite being able to FAIL. A suite that only ever passes would let a future backend ship a no-op CAS and look conformant — the same class of problem as RR-SG8P1N on the sibling branch, where a test that could not fail let a dead retry loop ship. VERIFIED by negative control, independently reproduced: sabotaging memstore''s updateEntityIf so the version comparison never runs (replacing `if !cond.IsZero()` with `if false`) makes internal/store/storetest''s CAS suite fail on multiple distinct tests, including TestConformance/CAS/RetryWithActualSucceeds ("An error is expected but got nil, expected: *store.VersionConflictError") and TestConformance/CAS/ConcurrentAppendersAllLandWithRetry, which reports "appender 2''s write was lost despite CAS" with the surviving appends enumerated. Restoring the comparison returns the package to green. The concurrent-appender test is the one that matters most: it does not merely assert an error type, it asserts that every concurrent append SURVIVES, which is the user-visible property (no lost writes) rather than a proxy for it. Nothing to fix. Noted so a later reader knows the suite was checked for teeth rather than assumed to have them.'
severity: nit
resolution: No change required. Verification performed during review; the sabotage was reverted and the package confirmed green afterwards.
status: addressed
---
