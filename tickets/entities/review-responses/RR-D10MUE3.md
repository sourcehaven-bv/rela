---
id: RR-D10MUE3
type: review-response
title: Dry run skips resolved choices
finding: A dry run does not apply resolved delete choices in memory, so its report can differ from the real run.
severity: minor
reason: Pre-existing shape of dry run (it skips every ledger rewrite); the real run is correct. Follow-up if dry runs of resolved deletes matter.
status: deferred
---
