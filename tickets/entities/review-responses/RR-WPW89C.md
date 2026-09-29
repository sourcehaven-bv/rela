---
id: RR-WPW89C
type: review-response
title: Incoming-edge error omits the source face
finding: The incoming-edge wrap printed rel.From without its tail face.
severity: nit
resolution: The wrap uses entity.FormatStateRef(rel.From, rel.FromFace).
status: addressed
---
