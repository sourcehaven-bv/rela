---
id: RR-EISJTE
type: review-response
title: fsstore replace+oversize destroys existing valid attachment (data loss)
finding: 'fsstore.AttachFile removes the OLD file before streaming the new one. If the new upload trips the backstop cap, the partial new file is cleaned up but the old (valid) attachment is already gone and the in-memory index points at a now-missing file → next ReadAttachment fails. Same-filename case truncates-in-place then deletes. The storetest RejectsOversize case doesn''t catch it (fresh property only). Fix: write new file first, only remove old after the streamed write succeeds. Add a replace-then-oversize conformance case asserting the original is still readable.'
severity: critical
resolution: Reordered fsstore.AttachFile to write the new bytes to a temp key (fileKey+'.new') FIRST, and only after a successful write remove the superseded old file and Rename temp into place. A failed write (e.g. backstop cap trip) now removes only the temp partial — the existing attachment and in-memory index are untouched. Fixes both the different-filename and same-filename (truncate-in-place) data-loss cases. Added storetest OversizeReplaceKeepsExisting (all backends) + dataentry TestAttachmentUpload_ReplaceOversizeKeepsExisting asserting the original is still readable after a failed oversize replace.
status: addressed
---
