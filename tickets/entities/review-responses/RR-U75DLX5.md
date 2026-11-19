---
id: RR-U75DLX5
type: review-response
title: start doc says it refuses while closing
finding: A run in the success hold is returned after Close, unlike the doc said.
severity: nit
resolution: start no longer returns a run from the hold; it refuses with errProvisionedRecently, and the start doc lists that refusal.
status: addressed
---
