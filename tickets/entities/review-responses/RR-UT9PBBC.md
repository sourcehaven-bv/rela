---
id: RR-UT9PBBC
type: review-response
title: Small cleanups
finding: 'Code review: checkFailed slices the check name by bytes; perRow is production code used only by tests; an unparsable violation id is sent to the resolver as a zero Ref.'
severity: nit
resolution: checkFailed takes the display name. perRow moved to redactpage_test.go. Unparsable ids are left out of the batch.
status: addressed
---
