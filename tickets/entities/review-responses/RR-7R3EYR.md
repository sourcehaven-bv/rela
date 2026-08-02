---
id: RR-7R3EYR
type: review-response
title: Re-encode must update filename extension to match new format
finding: 'The plan re-encodes (JPEG→PNG, and every WebP→PNG/JPEG since WebP has no pure-Go encoder) but did not specify updating the stored filename extension. A .webp file holding JPEG bytes breaks the extension-derived download Content-Type (contentTypeForFilename, handlers_attachment.go) which with X-Content-Type-Options: nosniff makes the browser honour the wrong type, and breaks the sniff↔extension polyglot check (mimecheck.go:111) on any re-validation.'
severity: significant
resolution: imgproc.Normalize returns Info.Format; the native-step wiring sets ProcessInfo.FileName to the input basename with the extension swapped to match the re-encode target (.jpg/.png). The seam already re-resolves a changed name (attachment.go:169 resolveAttachName) for collision/cap. Added to plan Design + a new AC.
status: addressed
---
