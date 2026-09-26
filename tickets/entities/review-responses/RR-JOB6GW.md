---
id: RR-JOB6GW
type: review-response
title: Narrow attachment interfaces do not narrow the snapshot field
finding: AttachmentSnapshot.Service is the concrete *attachment.Service.
severity: nit
reason: The snapshot is built only by NewAttachmentSnapshot; handlers bind the reader/writer interfaces at use so read tools cannot call write methods by accident. Exporting a field of unexported interface type would be worse API.
status: wont-fix
---
