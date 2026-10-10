---
id: RR-BPSZLB
type: review-response
title: RelationRecordIDReader breaks the sqlitestore method limit and the VersionStore design
finding: The new RelationRecordIDReader store interface pushed sqlitestore over the plimsoll method limit and contradicted the VersionStore design.
severity: significant
resolution: Fixed in 3b2b0bdd8. RelationRecordIDReader is removed. Lineage ids are read before the transaction through VersionStore, and the race is documented. TestReplaceRelations_PassesLineageIDToRecorder checks that the RecordID reaches the recorder.
status: addressed
---

Review finding R2-3.
