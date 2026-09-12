---
id: RR-HNQ4Q9
type: review-response
title: 'liveRelationHash omits FromFace from the hash input, so it is correct only by coincidence'
finding: 'internal/store/sqlitestore/purge.go: the query pins from_face = '''' but the RelationVersionInput handed to contentHashOfRelation left FromFace unset, even though canonical.HashRelation folds the tail into the hash. The zero value happens to be right while that predicate holds, so the code is correct today by coincidence rather than by construction. The entity counterpart two functions up explicitly passes Face and carries a comment explaining that omitting it would suppress a sibling face''s capture. If anyone relaxes the from_face predicate, this silently produces a hash that suppresses the wrong lineage''s sweep capture. pgstore has the same omission.'
severity: significant
resolution: 'Carried FromFace from the scanned row, mirroring the entity hash, with a comment stating that the zero value is right today only because of the predicate above it — so the next reader does not have to re-derive that.'
status: addressed
---
