---
id: RR-GQ8Y83
type: review-response
title: A face move silently destroyed the row's outgoing relations
finding: 'Pre-existing and independent of the atomicity work, found while checking the reviewer''s version-capture question. DeleteEntityState takes a face''s OUTGOING edges with it — they were written against that face and nothing else can own them — and the CreateEntity at the destination copies the row''s content, not its edges. So every migrate_face and rename_face move destroyed the relations its row owned: no error, no history row, and `rela analyze` reports a graph that is merely smaller rather than broken. The DeleteResult naming exactly what was destroyed was being discarded with `_`.'
severity: critical
resolution: 'Reproduced against the existing test fixture: TSK-1 --assigned-to--> PER-1 present before the migration, 0 relations after. applyFaceMove now reads DeleteResult and re-creates each outgoing edge on the destination face via CreateRelation with RelationData.FromFace, skipping anything not tailed on the moved face so a wrong-tail edge cannot be invented. Verified: the edge now moves with the row (TSK-1@"" becomes TSK-1@"draft"). Pinned by TestMigrateFace_CarriesOutgoingEdgesToTheNewFace.'
status: addressed
---

The reviewer raised this as part of a question about version capture, and the
capture question turned out to be the less important half. I had reasoned that a
move loses nothing because the content survives at the new coordinate — true for
the row, false for its edges, and I would not have checked without the prompt.

The tell was in the code the whole time: `if _, err :=
s.DeleteEntityState(...)`. The store returns what it destroyed and the call
discarded it.
