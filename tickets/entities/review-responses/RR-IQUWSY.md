---
id: RR-IQUWSY
type: review-response
title: Relation target url ignores the served face
finding: neighbor built the url from the edge ref (a bare id) while the title came from the face the world serves.
severity: significant
resolution: neighbor now builds the url from the served header's id and face.
reason: The url now names the served face (h.ID and h.Face), like pile items and summaries.
status: addressed
---
