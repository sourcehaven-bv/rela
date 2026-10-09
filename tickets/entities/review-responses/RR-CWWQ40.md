---
id: RR-CWWQ40
type: review-response
title: Remove endpoint is an existence oracle for hidden refs
finding: _remove accepted unreadable refs and echoed {removed:[...]}; a hidden-but-existing ref is still stored while a deleted one was dropped by EntityDeleted, so the echo distinguished hidden from deleted.
severity: critical
resolution: 'Plan: _remove returns 204 with no body regardless of what was stored. Undo re-adds only what the SPA showed. Handler test asserts byte-identical responses for a hidden ref, a deleted ref and a never-added ref.'
status: addressed
---
