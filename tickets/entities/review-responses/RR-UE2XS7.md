---
id: RR-UE2XS7
type: review-response
title: Attachment-dir abort under-reports the entity - and my comment asserts it cannot happen
finding: By that point the entity file is already removed so DeletedEntities empty is the same denial this ticket fixes
severity: critical
resolution: 'Confirmed - my bug and my false comment. Fixed by making attachment-dir removal NON-FATAL: by that point the entity file and every relation file are gone so the delete has materially succeeded; failing it would report failure for an operation that worked and leave the log denying a real deletion. An orphaned attachment dir is the lesser evil (analyze already reports those) and removeAttachmentDir prunes its index before any fallible I/O so the index stays consistent. The abort-path comment now states an invariant that is exactly rather than approximately true. Pinned by TestDeleteEntity_AttachmentDirFailure_StillSucceeds; verified it fails with the old return-error behaviour.'
status: addressed
---

The third abort path is `removeAttachmentDir`. By the time it can fail:

1. every relation file is removed,
2. **the entity file has already been removed** — `s.rooted.Remove(key)` on the
line above succeeded,
3. `removeAttachmentDir` then fails.

My code returns `&store.DeleteResult{DeletedRelations: removed}, err` with
`DeletedEntities` empty, and the comment I added claims:

> DeletedEntities stays empty on every abort path below: the entity file is
> either untouched or its removal is what failed.

**False for this path.** The entity file's removal did not fail — it succeeded.
So the entity is gone from disk and the log denies it: precisely #929's own
failure mode, reproduced one layer down by the fix for #929.

Worse, `removeAttachmentDir` (`attachment.go:185-192`) prunes the in-memory
`s.attachments` index **before** any I/O that can fail. On this path the store
returns an error having already dropped the entity file, every relation file,
and the attachment index entries — while reporting only relations.

I mutation-tested the relation path and not this one, which is exactly why the
false comment survived.
