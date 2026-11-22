---
id: RR-3P8YW21
type: review-response
title: Face-limited grants refused even when the selection admits one face
finding: scopeFaces ignored q.Faces, so a world admitting one face still refused.
severity: significant
resolution: 'ListEntityPage intersects the read scope''s FaceIn with the faces the page selection can return (returnableFaces: AtFaces, or a world chain with exclude fallback). The remaining refusal (selection over several faces) is documented in GUIDE-mcp-server.'
status: addressed
---
