---
id: RR-CLGENR
type: review-response
title: Incoming edges are always fetched even when no column needs them
finding: gatherListPeers dropped the needIn/needOut check and read DirectionBoth for every export. On a type with heavy fan-in the unused incoming edges are loaded and filtered for nothing over up to 5000 rows.
severity: significant
resolution: Added entityReader.pageRelationsDir with a direction argument. The export computes needIn/needOut and reads Outgoing or Incoming or Both in one query. Face filtering runs only for a direction a column uses. TestPageRelationsDir pins the split.
status: addressed
---
