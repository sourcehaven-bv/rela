---
id: RR-Y0AD4M
type: review-response
title: '[security] Deleted-face timeline checked the face grant against the URL type'
finding: 'Security review: for a fully deleted id nothing tied the URL type to the lineage''s type, so a face grant on one type opened another type''s timeline metadata.'
severity: minor
resolution: serveHistoryTimeline refuses a deleted-face timeline unless every version is of the URL type. Pinned by TestFacedHistory_DeletedFaceGates/lineage_of_another_type.
status: addressed
---
