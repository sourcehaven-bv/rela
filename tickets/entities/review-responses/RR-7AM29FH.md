---
id: RR-7AM29FH
type: review-response
title: Remembered space is per browser, not per user
finding: On a shared browser user B lands in user A's last space when B may enter it.
severity: nit
reason: 'Only a hint: the server resolves it against B''s permitted spaces and space ids are config, not secrets. The SPA has no stable principal id before the sidebar loads.'
status: wont-fix
---
