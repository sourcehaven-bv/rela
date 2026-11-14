---
id: RR-Z2WEN5L
type: review-response
title: ListMarked is still read in full on create
finding: markedIDs and the unique check call ListMarked, which loads every soft-deleted family, and Counting does not count it.
severity: minor
reason: Pre-existing read; soft-deleted families are few. Belongs with the follow-up on store lookups (TKT-GIAGLQ3).
status: deferred
---
