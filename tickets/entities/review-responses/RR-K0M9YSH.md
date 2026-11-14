---
id: RR-K0M9YSH
type: review-response
title: A relation error did not leave the record out
finding: Relation errors were found after the create or patch was planned, so --skip-errors still wrote the entity.
severity: significant
resolution: A settle pass resolves every required relation before any write is planned and drops failing drafts until nothing changes, so dependents of a dropped draft fail too (TestRelationErrors_DropRecordsInChain).
status: addressed
---
