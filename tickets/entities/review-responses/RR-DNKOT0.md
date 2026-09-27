---
id: RR-DNKOT0
type: review-response
title: Blob MIME type passes active content labels through
finding: An SVG or other active type went out as an embedded resource labeled with its real MIME type; a client could render it.
severity: minor
resolution: passiveBlobTypes keeps only PDF/JSON/plain text/CSV; everything else is application/octet-stream. Covered in TestAttachmentContent.
status: addressed
---
