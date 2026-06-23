---
id: RR-ROF51F
type: review-response
title: _attachments leaks files of policy-hidden file properties (read-path ACL bypass)
finding: 'computeAttachments iterates store.ListAttachments directly and emits every property''s files with working download hrefs, ignoring the field-visibility verdicts (FieldVerdicts.Visible) that stripHiddenProperties/_fields enforce. handleV1GetAttachment gates only on entity-read + isFileProperty, never on Visible. So in an ACL deployment with field-level hide policy, a viewer who can read the entity but for whom a file property is hidden still sees the file in _attachments AND can download its bytes via the per-file URL. Only became reachable when _attachments joined the response next to the existing hidden-field machinery. Fix: compute field verdicts once; skip hidden props in computeAttachments AND 404 them in handleV1GetAttachment + the write preflight (both ends — hiding from the map alone leaves the guessable URL downloadable).'
severity: critical
resolution: 'Both ends now enforce field visibility. attachEntityAffordances resolves FieldVerdicts once and passes them to computeAttachments, which skips any property where !IsVisible(prop) (so a hidden file property''s files never appear in _attachments). handleV1GetAttachment and attachmentWritePreflight now call a.isPropertyHidden(ctx, entity, property) and 404 a hidden property (so the guessable per-file URL can''t download it either). Added TestAttachment_HiddenPropertyNotLeaked: with a fakeResolver hiding ''screenshot'', _attachments omits it AND the per-file download 404s.'
status: addressed
---
