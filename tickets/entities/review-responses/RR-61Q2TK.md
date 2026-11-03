---
id: RR-61Q2TK
type: review-response
title: Attachment temp leftovers are never swept
finding: 'internal/store/fsstore/attachment.go:80-84: the attachment temp name is now random, so a crash mid-upload leaves a .rela-attach-tmp-<hex> file (up to store.MaxAttachmentBytes, 64 MiB) that no later upload overwrites. Before, the name was a hash of the file name, so re-uploading the same name reused it. Nothing removes it: cleanupTempFiles walks only entities/ and relations/, FindOrphanedTempFiles likewise, and loadPropertyAttachments only skips the prefix. Fix: in cleanupTempFiles (or rela gc --temp-files) also walk the attachments key and remove names with attachTempPrefix whose mtime passes storage.IsStaleTemp; add a test next to TestOpen_TempFileSweepHonoursGracePeriod.'
severity: minor
resolution: Opening a store removes stale .rela-attach-tmp- files (IsStaleTemp) in loadPropertyAttachments, and rela gc --temp-files scans attachments/; the prefix is defined once in internal/storage. Tested by TestOpen_AttachmentTempSweepHonoursGracePeriod and new gc cases.
status: addressed
---
