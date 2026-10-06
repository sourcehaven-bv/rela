---
id: RR-XXQGNG
type: review-response
title: recordIDIsHeadOfKey ignores from_face on pg and sqlite
finding: The record-id membership check behind relation purge (RecordID selector) and relation history reads (?record_id=) matched from/type/to only, so a sibling tail's record id passed and reached that tail's lineage.
severity: significant
resolution: recordIDIsHeadOfKey takes the tail face and filters rv.from_face on both backends; purge passes req.FromFace and resolveLineageIDs passes q.FromFace. New storetest subtest RelationRecordIDIsBoundToItsTail (ErrNotFound for both the read and the purge; draft tail intact) passes on sqlite and pg.
status: addressed
---
