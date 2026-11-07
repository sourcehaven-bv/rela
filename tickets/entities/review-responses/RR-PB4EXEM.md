---
id: RR-PB4EXEM
type: review-response
title: Saver election trusts client awareness
finding: 'Plan: the writable peer with the lowest awareness clientID saves. Awareness is client-written, so a peer that advertises itself writable but cannot save (or a crashed saver whose awareness has not timed out, up to 30s) stops all saves. Plan change: any writable peer saves when the live document''s hash is not in the room''s saved map 5s after the saver''s debounce would have fired.'
severity: significant
resolution: 'Addressed in plan: fallback saver when the live hash is not in saved 5s after the saver debounce.'
status: addressed
---
