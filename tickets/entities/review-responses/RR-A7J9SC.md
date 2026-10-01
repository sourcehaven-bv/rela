---
id: RR-A7J9SC
type: review-response
title: Pages of 500 rows include content
finding: A ListEntities page holds up to 500 bodies at once.
severity: minor
reason: Bounded and tunable in one constant; headers-only callers use ListEntityHeaders. The end-to-end run on the perf project showed no issue.
status: wont-fix
---
