---
id: RR-MV5EM0
type: review-response
title: Reader paths panic on nil slice elements while tracer paths guard nil
finding: Filter/FilterRelations/redacted dereference c.Type/c.ID/rel.From with no nil check; collectNodeIDs/rebuild in the tracer decorator do guard nil. A nil candidate is a caller bug, but in a fail-closed security path a panic is the wrong failure mode (an upstream recover() could skip filtering entirely). Drop nil elements defensively, consistent with the tracer.
severity: minor
resolution: Filter and FilterRelations now drop nil elements fail-closed (no panic), consistent with the tracer decorator's nil guards. Suite case NilElementsDroppedNotPanicked pins it for both surfaces.
status: addressed
---
