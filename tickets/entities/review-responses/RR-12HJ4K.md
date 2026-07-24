---
id: RR-12HJ4K
type: review-response
title: Flush decision must be a pure, unit-testable seam (not buried in UpdateEntity)
finding: The sweep runs on a real ticker goroutine, so a flush test must drive two updates with different attributions and assert the synchronous pre-edit version without a sweep interfering. Design the flush decision as a pure function (probe result + incoming attribution → decision) callable in isolation, with integration tests on top — a design constraint for the flush follow-up, not just a test note.
severity: minor
reason: Flush split into follow-up TKT-0IGI4V; pinned there as 'Testable seam' (pure decision function).
status: deferred
---
