---
id: RR-HBER2
type: review-response
title: loadAttachmentsIndex reports ciphertext size on reopen under encryption
finding: 'fsstore.go:242-252 caches info.Size() from fileEntry.Info() — that''s the on-disk ciphertext size. Yet AttachFile (attachment.go:53) stores int64(len(data)) (plaintext). So on first write: plaintext size; after store reopen: ciphertext size (~229 bytes age overhead). ListAttachments returns that via store.AttachmentInfo.Size — silent contract violation. Mitigated today because FSFactory never sets AttachmentsDir, but the bug compiles and passes tests; breaks the day someone wires attachments in production.'
severity: significant
resolution: 'loadAttachmentsIndex now reads each attachment via s.bytes.ReadFile and sets size to len(plaintext). Stat-based sizing removed. Regression test TestFSStore_Encrypted_AttachmentSizeOnReopen covers the case: write attachment, Close, reopen, ListAttachments must report plaintext size.'
status: addressed
---
