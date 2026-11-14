---
id: RR-GH9WPTH
type: review-response
title: One gate call per row
finding: missing() and matching call GetAddress once per stored row.
severity: minor
reason: Calls are cached per address and bounded by the rows of the imported types. The plan needs the redacted row for values, which the header-only batch API does not give. Follow-up if imports of large types prove slow.
status: deferred
---
