---
id: RR-VSYUX9
type: review-response
title: Per-property lock keys pin several postgres connections
finding: The lock was keyed (id, property). A write touching K file properties held K keys, and on postgres each held key pins a pool connection.
severity: significant
resolution: 'One key per entity: AttachmentLockKey(id). The attachment service, copy engine and face delete share it.'
status: addressed
---
