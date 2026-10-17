---
id: RR-NANY6J
type: review-response
title: EdgesInto read every edge into shared ancestors
finding: StoreGraph.EdgesInto filtered on the target only. A project every task is in can hold edges from thousands of non-members, and the batch read all of them.
severity: significant
resolution: Added store.RelationQuery.PeerIDs (filters the other endpoint) to pgstore, sqlitestore and the fs/mem matcher, with a storetest conformance case that passes on all four backends. EdgesInto became EdgesBetween(members, targets).
status: addressed
---
