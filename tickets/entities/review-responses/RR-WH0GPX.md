---
id: RR-WH0GPX
type: review-response
title: Cursor digests are unkeyed and not bound to a principal
finding: 'Security design review: plain truncated sha256 in a cursor that ends up in transcripts lets anyone holding it test guesses offline against low-entropy results, and the cursor works for any principal.'
severity: minor
resolution: Digests are HMAC-SHA256 with a random per-process key over the principal's ShareKey, the target key and the result. A cursor from another process or principal reports every target changed once, which is safe.
status: addressed
---
