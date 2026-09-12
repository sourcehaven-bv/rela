---
id: RR-EISIAL
type: review-response
title: 'Purge deletes rows and writes its sweep-suppression tombstone in two separate autocommit statements'
finding: 'internal/store/sqlitestore/purge.go: PurgeVersions ran deletePurgeTargets and writeEntityPurgeTombstone as two statements on the pool, each autocommitting. A crash, a cancelled context or a failed tombstone insert between them leaves the history hard-deleted with no tombstone, so the sweep re-captures the live content within one interval and the irreversible erasure silently undoes itself. versionMu provides mutual exclusion against a concurrent sweep, which is a different property and survives neither a crash nor an early return. PurgeRelationVersions had the same shape plus an extra hazard: it resolved liveRecordID AFTER the delete had already committed.'
severity: critical
resolution: 'Widened the four purge helpers from *sql.DB to the existing querier interface (which version.go and relation_version.go already took) and wrapped the mutating half of both purge methods in one transaction via a new purgeTx helper. The delete and the tombstone now commit together or not at all, and neither the row count nor TombstoneWritten is applied to the result until the commit succeeds, so a rolled-back purge reports nothing purged rather than a count for rows that are still present. liveRecordID moved ahead of the delete. Verified by the Version/Purge conformance cases on both backends.'
status: addressed
---
