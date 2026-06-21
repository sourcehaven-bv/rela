---
id: RR-N96YV0
type: review-response
title: max==1 replace deletes old file before attaching new (data-loss window)
finding: 'At max==1 the upload handler (and CLI Service.Attach) DeleteAttachment(old) THEN AttachFile(new). If AttachFile fails for any reason other than the early header.Size guard (store I/O error, disk full, mid-stream read error, streaming cap on an under-declared part), the old bytes are gone and the new never land — original lost, frontmatter stale. fsstore temp-then-swap only protects same-name overwrite, not a different-name replace (two separate store ops). TestAttachmentUpload_ReplaceOversizeKeepsExisting gives false confidence: its 413 short-circuits at header.Size BEFORE the delete block, so the hazard is never exercised. Fix: AttachFile first, then delete siblings whose name != new name, then re-stamp — in BOTH the handler and the CLI service. Add a test that fails AttachFile via a store error (not the size guard) and asserts the original survives.'
severity: critical
resolution: Reordered to attach-first-then-delete-siblings in the shared attachment.Service.WriteAttachment (used by both the HTTP handler and CLI). The new file is written FIRST; only after it lands are the other files removed (at max==1). A store/read failure mid-write now leaves the existing attachment intact. Added TestService_ReplaceFailureKeepsExisting using a failingReader that errors mid-stream (NOT the size guard) and asserts the original survives — the test that actually exercises the hazard. Verified live (upload+delete-by-href round-trip).
status: addressed
---
