---
id: RR-E0PJ0B
type: review-response
title: RelationQuery.EntityIDs must compose with Direction and FromFace exactly like EntityID
finding: RelationQuery already carries FromFace (store.go:434) and Direction. A batch that ignores FromFace returns state-tailed edges a per-row call excluded. sqlitestore (relation.go:110-120) also implements EntityID and needs the plural.
severity: minor
resolution: Plan defines EntityIDs as the plural of EntityID under identical Direction and FromFace semantics, implemented in fs/mem/pg/sqlite, with a storetest case asserting {EntityIDs:[a,b], FromFace:&f} equals the union of the two scalar calls, and nil vs empty pinned.
status: addressed
---
