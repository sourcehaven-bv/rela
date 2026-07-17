---
id: RR-1CZUZB
type: review-response
title: Relation tombstone resume wedges pull (not-found sentinel mismatch)
finding: isLocalNotFound only matched entitymanager.ErrEntityNotFound, but Manager.DeleteRelation wraps store.ErrNotFound directly (a different sentinel). A re-played relation tombstone for an already-absent local relation would abort the entire pull instead of being a no-op, wedging sync on resume. Tests missed it because the fake applier returned the same unmatched error and no test deleted an already-absent relation.
severity: critical
resolution: isLocalNotFound now also matches store.ErrNotFound and entitymanager.ErrRelationNotFound. Added regression test TestPull_RelationTombstone_IdempotentOnResume that mirrors a relation tombstone, rewinds the cursor, and re-pulls, asserting no error.
status: addressed
---
