---
id: RR-8SRRU9
type: review-response
title: Relocated ?include= / ?q= notes were orphaned from the machinery they guard
finding: The two historical "do not restore this refusal" notes were moved into refuseWorldIncapablePath's doc, but that function has nothing to do with either parameter — it inspects r.URL.Path and a world name, never a query param. So an 11-line path predicate carried two paragraphs about search threading and neighbor resolution, under a heading admitting they merely 'used to stand beside' it. A reader who breaks world-scoped search will be reading search code, not a path predicate.
severity: minor
resolution: Filed each note with its machinery instead. The `?q=` warning went to queryService.freeTextIDsForType (queryservice.go), whose doc already explains the world-scoped search and the denied-world seam — only the 'do not restore without reverting this threading' clause was missing. The `?include=` warning went to the worldNeighbors type doc (worldneighbors.go), which already records RULING 12 and the ungated default-world entityReader history. Both shrank to three lines because the surrounding doc already carried the substance; only the warning was new.
status: addressed
---
