---
id: RR-EV48RR
type: review-response
title: File-property value is a writer-controlled capability to read another face's bytes
finding: A writer on face B can set B's file value to a name face A uploaded and download A's bytes because bytes are keyed per entity and the download gate trusts the face's value
severity: critical
resolution: 'Design section 8.1 (PR 4): generic writes must leave file-type values unchanged and creates must omit them (422 otherwise); only the attachment service (dedicated StampAttachments manager method) and copy engine and sync and data migration may change them; restore keeps live file values. Tests pin the cross-face download attempt as 404.'
status: addressed
---

**Where:** design section 4, "Download".

The design authorizes a download when `fileName ∈ referencedNames(face)`. The
face's file-property value is therefore the capability to read bytes. But that
value is an ordinary string property (`metamodel.PropertyTypeFile`,
`validateFileValue` checks only "string(s)"), writable through any
`PatchEntity`/`UpdateEntity`/Lua write that holds write on that face.

Bytes are keyed per entity, not per face. A principal who may write face B but
may not read face A can set B's value to a name that A uploaded, then download
A's bytes through B. That discloses hidden-face content, which CLAUDE.md classes
as secret. A history restore of an old B version can do the same by coincidence
when a later A upload reuses the name.
