---
id: RR-C3M3BR
type: review-response
title: Export endpoint must copy attachment download hardening (nosniff, sandbox CSP, sanitized filename, gate-before-render)
finding: 'A transform produces attacker-influenceable bytes (PDF/DOCX from user content). Just setting Content-Type is insufficient. The existing attachment download (handlers_attachment.go) sets X-Content-Type-Options: nosniff, Content-Security-Policy: sandbox; default-src ''none'', sanitized Content-Disposition via safeAttachmentFilename/unsafeFilenameRe, and runs the ACL read gate before store access. Filename derived from entity title/id would otherwise allow header injection / weird filenames.'
severity: significant
resolution: 'Export endpoints mirror the attachment download pattern exactly: nosniff + sandbox CSP + Cache-Control: no-store + Content-Disposition with a sanitized filename (reuse the unsafeFilenameRe/safeAttachmentFilename helper), gate-before-render. Filename derived from a sanitized entity id/list-type, never raw title bytes into the header.'
status: addressed
---
