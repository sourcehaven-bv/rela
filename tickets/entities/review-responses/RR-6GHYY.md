---
id: RR-6GHYY
type: review-response
title: Anonymous struct{from,relType,to string} repeated 7 times; shadows existing removedEdge
finding: |-
    api_v1.go:544, 691, 704, 837, 849, 885, 888 — same anonymous struct in 7 places. workspace/tx.go:48 already has `removedEdge` with the exact same shape.

    Fix: define `type relationEdge struct { from, relType, to string }` in api_v1.go (or export workspace.RemovedEdge). Replace all occurrences.
severity: minor
resolution: Defined relationEdge type at package level. All 7 occurrences of the anonymous struct{from,relType,to string} replaced with relationEdge. relationDiff also lifted to package level so helper functions can take it as a parameter.
status: addressed
---
