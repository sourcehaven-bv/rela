---
id: RR-3EI8RHL
type: review-response
title: No test for Host.Close cancelling an in-flight run
finding: The documented behaviour that host shutdown cancels provisioning runs in progress and waits for them had no test.
severity: minor
resolution: 'Fixed in 13cd61e18: added TestProvision_CloseCancelsAndWaits.'
status: addressed
---
