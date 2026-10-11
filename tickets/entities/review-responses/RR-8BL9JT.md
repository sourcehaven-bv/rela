---
id: RR-8BL9JT
type: review-response
title: liveRelationHash hand-writes the column list
finding: 'sqlitestore and pgstore liveRelationHash repeat the relation column list; the same drift risk as #1807.'
severity: significant
resolution: sqlitestore uses relationColumns; pgstore hoists GetRelation's query to getRelationSQL and reuses it.
status: addressed
---
