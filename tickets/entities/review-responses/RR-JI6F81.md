---
id: RR-JI6F81
type: review-response
title: Return SVG as blob, not image content
finding: 'image/svg+xml is active content. Fix: image content only for png/jpeg/gif/webp; everything else as an embedded blob.'
severity: nit
resolution: inlineImageTypes lists png/jpeg/gif/webp only; SVG goes out as an embedded blob. Covered in TestAttachmentContent.
status: addressed
---
