---
id: RR-P5EFFT
type: review-response
title: Attachment handlers load a.State() 4x — snapshot-drift on metamodel reload
finding: 'handleV1PutAttachment calls a.State() separately in maxAttachmentBytes, attachmentWritePreflight->isFileProperty, attachmentService (Meta: a.State().Meta), and filePropertyDef. CLAUDE.md ''capture state once per operation'' — a reload mid-handler can make the handler gate on one snapshot''s PropertyDef while the on-demand Service re-reads a different snapshot''s FileMax(), so the cap the handler enforces and the cap the service enforces disagree. Fix: capture s := a.State() once at the top and thread it (or the resolved Meta + propDef) into all four.'
severity: significant
resolution: 'handleV1PutAttachment/handleV1DeleteAttachment now capture s := a.State() once and thread it: maxAttachmentBytes(s), filePropertyDef(s, ...) (now a package func taking the snapshot), and attachmentService(s) (Meta from the same snapshot). The handler''s gating def and the service''s FileMax() now come from one metamodel snapshot, so a mid-handler reload can''t make them disagree.'
status: addressed
---
