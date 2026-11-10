---
id: RR-AD1RQOD
type: review-response
title: rivermigrate MigrateTx is deprecated
finding: 'staticcheck SA1019: River deprecates MigrateTx because some migrations cannot share one transaction. The queue ran River migrations inside the transaction that held the setup advisory lock.'
severity: minor
resolution: 'Addressed: migrateRiver holds the same advisory lock at session level on its own connection and calls Migrate. A failed unlock closes that connection, which releases the lock.'
status: addressed
---
