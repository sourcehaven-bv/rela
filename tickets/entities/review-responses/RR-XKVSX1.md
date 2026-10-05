---
id: RR-XKVSX1
type: review-response
title: ownedEdges fail-closed comment overstated what happened on error
finding: On a header read error the loop broke but kept the sources resolved before the error.
severity: minor
resolution: On error ownedEdges now clears every resolution from the batch, so every edge that needed it is dropped, matching the doc.
status: addressed
---
