---
id: RR-RUIBRQ
type: review-response
title: session_notice is listed under actions in the errors table
finding: 'docs/realtime.md:781 (and GUIDE-realtime.md:787): the 409 row reads "For actions: the message is a session notice (session_notice)". The 409 comes from Edit and Delete (ops.go), not from actions. Fix: move it into the general part of the row, e.g. "an edit or removal targets a session notice (session_notice)". Then regenerate with just docs.'
severity: minor
resolution: session_notice and the new session_message sit in the edit/remove part of the 409 row; docs regenerated.
status: addressed
---
