---
id: RR-CIH3TZ
type: review-response
title: Handler and CLI duplicate the whole max-policy (already drifting)
finding: resolveUploadFileName vs resolveAttachName; stampAttachmentProperty vs stampProperty; attachmentFileNames in both dataentry and attachment packages; two path-builders (attachmentKey vs attachPath) computing the same string. Two copies of security-relevant policy (cap enforcement, suffixing, scalar-vs-list stamping). Already diverged (the handler has a dead name=='' check the CLI omits). A future change to the stamping rule will update one and miss the other. Consolidate into the attachment package (which has the store dep) and have the handler call it.
severity: significant
resolution: 'Consolidated the entire max-policy into attachment.Service: WriteAttachment (cap/suffix/attach-then-delete/stamp) and DeleteAttachment (per-file delete + re-stamp). The HTTP handler now builds the service on demand (a.attachmentService(), cheap struct wrapper over a.store/a.entityManager/a.State().Meta) and delegates; the CLI Attach/Detach already call the same methods. Removed the duplicated handler helpers (resolveUploadFileName, stampAttachmentProperty, attachmentFileNames, filePropertyMax, attachmentKey, the uploadResolveCode enum). Single source of truth for the policy. Added attachment to dataentry''s allowed deps in .go-arch-lint.yml (peer of entitymanager); arch-lint passes.'
status: addressed
---
