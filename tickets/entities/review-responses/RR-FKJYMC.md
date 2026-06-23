---
id: RR-FKJYMC
type: review-response
title: _attachments leaks onto mutation responses; docs claim GET-only
finding: computeAttachments runs in attachEntityAffordances, which serializeEntityForWire calls for PATCH/POST/clone/dry-run create/custom-view — not just GET. But the V1Entity.Attachments comment, docs/data-entry/api-reference.md, and frontend Entity._attachments comment all assert 'absent on mutation responses'. Contract violation + a wasted ListAttachments roundtrip per dry-run keystroke. The pinning test only checks list-row omission, never a mutation response — false confidence.
severity: critical
resolution: 'Chose the consistent option: _attachments rides every per-entity response (GET/PATCH/POST/clone) like _fields/_relations, rather than restricting to GET. Fixed all three doc/comment sites (V1Entity.Attachments godoc, frontend Entity._attachments comment, docs/data-entry/api-reference.md) to state this accurately. Added TestAttachment_MetadataOnMutationResponse asserting the shared serializeEntityForWire output carries the map. Also added an early-return in computeAttachments when selfHref is empty so dry-run/odd shapes can''t emit a broken href.'
status: addressed
---
