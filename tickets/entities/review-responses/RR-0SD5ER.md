---
id: RR-0SD5ER
type: review-response
title: Copy and single-face delete bypass the attachment lock and reference count
finding: Only the attachment service runs eager ref-counted deletion under the lock; face copy and single-face delete change references without it causing dangling references and new orphans
severity: significant
resolution: 'Design section 8.4 (PR 4): copy engine takes the attachment lock per copied file property through a consumer-side AttachmentLocker supplied from lock.For(st); single-face delete runs the reference count under the lock. Generic writes can no longer remove names (8.1). Race test under -race.'
status: addressed
---

**Where:** design section 4 and risk 4; ruling 4.

Ruling 4 deletes shared bytes eagerly, under the attachment lock, when the last
face stops referencing them. The design only runs that step in the attachment
service. Other paths change which faces reference a name without the lock or the
reference count:

- the copy engine (`internal/entitymanager/copy.go`, `fields: all`) adds
references from a source face to a target face;
- deleting one face removes that face's references;
- a generic write that removes a name.

A copy that reads face A while a concurrent delete removes A's last reference
writes a dangling reference to deleted bytes. A single-face delete leaves new
orphans, which ruling 5 says eager deletion stops.
