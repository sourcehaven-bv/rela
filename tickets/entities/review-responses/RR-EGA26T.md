---
id: RR-EGA26T
type: review-response
title: Family-anchored channel types on a faced entity type are not in the test table
finding: 'A family-anchored pair uses InWorld(default world) in the holder query, and the anchor read serves the world-first readable face. They agree today because FaceIn applies before ranking and props after (graphquerynaive), but nothing pins it for the SQL backends. Fix: add AC 6 and parity-table rows for a family-anchored type on a faced entity, where the second-ranked face names the caller and the first does not. Also add a face-anchored channel on an unreadable face of a readable id.'
severity: minor
resolution: AC 6 and the parity table add a family-anchored type on a faced entity whose second-ranked face names the caller, and a face-anchored channel on an unreadable face of a readable id.
status: addressed
---
