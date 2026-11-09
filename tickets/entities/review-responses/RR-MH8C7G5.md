---
id: RR-MH8C7G5
type: review-response
title: Adopted relations and entities deleted on gone
finding: 'Rows that existed locally before the pull adopted them were deleted under on_gone: delete.'
severity: minor
resolution: 'Ledger rows carry an adopted flag; on_gone: delete detaches adopted rows with a kept issue. TestRun_AdoptedRowsAreNotDeleted.'
status: addressed
---
