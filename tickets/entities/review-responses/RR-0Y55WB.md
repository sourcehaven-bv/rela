---
id: RR-0Y55WB
type: review-response
title: Export download filename uses the request spelling
finding: exportBaseName uses the request id rather than the resolved address, so a bare id exports a face under a name without the face.
severity: nit
resolution: The export filename uses the resolved entry address; exportBaseName's comment updated.
status: addressed
---
