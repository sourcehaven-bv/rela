---
id: RR-6E9I3M
type: review-response
title: Attachment temp name has the same two-writer collision; the out-of-scope reason does not address it
finding: AttachFamilyFile (internal/store/fsstore/attachment.go) uses a deterministic temp key, attachTempPrefix + sha256(fileName)[:8], in a directory per entity and property, opened with truncation by OpenForWrite. Two processes attaching the same file name to the same entity share that temp file, which is failure 2 of this ticket. The plan says it is out of scope because attachments use their own prefix. The prefix only prevents collisions with the cleanup, not with a concurrent writer. Either add a random part to the attachment temp key (the prefix check in fsimport reconcile and loadAttachmentsIndex still matches), or say plainly that concurrent same-name attachment writes stay unsafe and why that is acceptable.
severity: minor
resolution: 'Plan updated: the attachment temp name gets a random part; AC8 tests two concurrent uploads of the same name.'
status: addressed
---
