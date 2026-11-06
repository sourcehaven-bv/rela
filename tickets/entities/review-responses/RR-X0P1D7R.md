---
id: RR-X0P1D7R
type: review-response
title: Test-only wrappers and unused channel parameters in realtime
finding: realtime keeps wrappers such as carriedRestarts and nextSlot with an unused channel parameter only for existing tests.
severity: minor
reason: Keeps the existing tests unchanged to prove no behavior change; M5 deletes the old paths.
status: deferred
---
