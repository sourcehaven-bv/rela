---
id: RR-QT7C5
type: review-response
title: Stale RelationHistoryReader godoc describes pre-#1127 create+delete rename model
finding: 'internal/store/pgstore/relation_version.go:97-100 still asserted that an endpoint rename ''rewrites a relation as create-new-triple + delete-old-triple ... so the new triple gets a FRESH rel_record_id''. Since #1127 rename is an atomic in-place UPDATE that keeps the rel_record_id; this is the primary design comment for relation lineage and contradicted the very behavior TKT-9TQ6I documents.'
severity: significant
resolution: Rewrote the godoc to state the atomic rename keeps the rel_record_id (continuous single lineage) and that relationLineageIDs is belt-and-braces for the historical/pre-#1127 forked path, mirroring the manager.go comment.
status: addressed
---
