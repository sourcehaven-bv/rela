---
id: RR-4N3YP2
type: review-response
title: Empty deleted lineage answered 200 while a hidden live face answered 404
finding: 'For a history:read holder, a face with no rows and no versions returned an empty 200 timeline, while a hidden live face returned 404: an existence oracle.'
severity: minor
resolution: serveHistoryTimeline refuses a deleted-face timeline that is empty (lineageOfType). Pinned by TestFacedHistory_DeletedFaceGates/no_lineage.
status: addressed
---
