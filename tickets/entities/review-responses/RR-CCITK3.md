---
id: RR-CCITK3
type: review-response
title: Relation restore must go through Manager.CreateRelation/UpdateRelation (endpoint check, validation, field gate) with a defined 409
finding: 'The design says relation restore is ''a normal authorized + validated + audited write'' but does not pin the path. It must call Manager.CreateRelation / Manager.UpdateRelation (not a raw store upsert) so the endpoint-existence check (GetEntity(from)/GetEntity(to), manager.go ~L702-709), ValidateRelation type check, and per-field write gate all fire. Restoring a deleted relation whose endpoint entity is now gone must map ErrEntityNotFound to a 409 (dangling-edge) with a clear message, NOT a 500. Note: there is NO relation field-write gate today (see RR-BZNL0S), so any claim of ''per-field write gate'' for relations must be reconciled with that reality — either state none exists or build it. Also add a separate RelationHistoryReader/RelationVersionWriter optional interface rather than fattening entity HistoryReader/VersionWriter (respects consumer-side-interface rule + plimsoll max-exported-methods=34 on pgstore.Store) (cranky S4, architect S2/M4).'
severity: significant
resolution: 'Design revised: restore goes through Manager.CreateRelation/UpdateRelation (endpoint check + ValidateRelation fire); dangling-endpoint maps to 409 not 500; separate RelationHistoryReader/RelationVersionWriter optional interfaces (not fattening entity ones, respects plimsoll). See ''Read/restore/ACL''.'
status: addressed
---
