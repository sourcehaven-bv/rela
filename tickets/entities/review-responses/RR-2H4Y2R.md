---
id: RR-2H4Y2R
type: review-response
title: Add-another and open-tab-two-links paths not in e2e
finding: The e2e spec does not cover linking with Create and add another or an open tab with two links for the type.
severity: minor
resolution: Not added.
reason: Both paths are client logic in pageCreateLink and SpaceCreateMenu and are covered by the unit tests from BUG-PFLS22. A browser test per branch adds a server start each without covering more of the server contract.
status: deferred
---
