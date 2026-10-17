---
id: RR-E8L9OJ
type: review-response
title: NeighborsForPage keyed rows by id
finding: page.rowIdx mapped id to one row index, so a page holding two faces of one entity gave edges to only the last one.
severity: minor
resolution: rowIdx maps id to every row index; ids are de-duplicated. The equivalence test adds a second face of POL-1 and fails on the old code.
status: addressed
---
