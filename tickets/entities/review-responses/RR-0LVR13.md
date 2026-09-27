---
id: RR-0LVR13
type: review-response
title: Split attachment read and write capabilities
finding: 'One four-method AttachmentFiles gives read tools Put/Detach. Fix: consumer-side attachmentReader{List, Open} and attachmentWriter interfaces; only write tools hold the writer.'
severity: minor
resolution: Consumer-side attachmentReader{List, Open} and attachmentWriter{WriteAttachment, DetachFile} in tools_attachment.go; read handlers only hold the reader.
status: addressed
---
