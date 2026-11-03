---
id: RR-64UKHA
type: review-response
title: The docs say an own post never counts as unread
finding: 'docs/realtime.md:1810 (and the mirror in GUIDE-realtime.md): "Your own post: Never". On the file backend the author''s marker moves best-effort after the records are durable (filechannels/reads.go markAfterWrite), and a failure is only logged, so the own post then shows as unread. Fix: add "(on the file store, a failed marker write after a stored post can leave it counted; the next mark clears it)", or soften the wording to "Not normally".'
severity: nit
resolution: Realtime guide states the file-store best-effort exception for own posts
status: addressed
---
