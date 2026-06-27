---
id: RR-81XSCY
type: review-response
title: Config limit above store backstop makes the 413 detail lie
finding: 'If app.max_attachment_bytes is configured ABOVE store.MaxAttachmentBytes (64MiB), a file in (64MiB, configuredLimit] passes the handler''s limitedContentReader but trips the store backstop, surfacing a 413 whose detail says ''maximum size is <configuredLimit> bytes'' — a lie; the real ceiling was 64MiB. Fix: clamp maxAttachmentBytes() to store.MaxAttachmentBytes so the configured limit can never exceed the backstop.'
severity: significant
resolution: maxAttachmentBytes() now clamps the configured/default limit to store.MaxAttachmentBytes, so the 413 detail can never promise a ceiling the store would reject. Added TestMaxAttachmentBytes_ClampsToStoreCap (default, lower-override, over-cap-clamped).
status: addressed
---
