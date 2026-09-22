---
id: RR-3QWH45
type: review-response
title: 'store.go grew 135 lines of migration machinery'
finding: 'BulkMigrator, the dispatcher and the fallback loop were added to the package''s central interface declaration file. The fallback is an implementation rather than a contract, and it is the longest function in the block.'
severity: minor
reason: 'Deferred to follow-up: it is a quality or ergonomics issue, not a correctness one, and it does not touch the data-loss paths this ticket exists to close. Filed rather than fixed so the reversal change stays reviewable.'
status: deferred
---
