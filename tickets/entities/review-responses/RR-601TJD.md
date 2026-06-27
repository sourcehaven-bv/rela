---
id: RR-601TJD
type: review-response
title: notFoundTitle duplicates gate literal; href empty-branch; silent ListAttachments error
finding: 'Cluster of minor correctness items: (a) notFoundTitle const and gateReadOrNotFound both hardcode ''Entity not found'' as independent literals linked only by a comment — no test asserts the two 404 bodies are byte-equal; (b) attachmentHref returns '''' when Self empty, producing a silently-wrong link — build from e.ID+plural directly instead; (c) attachmentFileName swallows ListAttachments errors as 404 on the byte-serving path, masking a backend outage — log before returning false. Plus nits: bytes.NewReader over string round-trip in test, ?.contentType?. optional chain.'
severity: minor
resolution: '(a) Hoisted a shared entityNotFoundTitle const in api_v1.go used by gateReadOrNotFound, handleV1GetEntity, and the attachment handler; added TestAttachment_DenyBodyMatchesGate asserting the attachment 404 body equals the gate''s 404 body (modulo instance). (b) computeAttachments builds the href from the canonical selfHref and now omits entries entirely if selfHref is empty (no broken-link branch). (c) attachmentFileName logs non-NotFound ListAttachments errors before returning false, so a backend outage on the byte-serving path isn''t silent. Plus nits: bytes.NewReader in the seed helper, ?.contentType?. optional chain in FileWidget. Also documented the ListAttachments+ReadAttachment TOCTOU window in the handler (harmless; collapse needs a store API change — separate ticket).'
status: addressed
---
