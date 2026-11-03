---
id: RR-VZZVKMD
type: review-response
title: rendererWith with a nil call panics
finding: rendererWith dereferences c; passing nil panics and passing the inbox call uses the last row's state.
severity: minor
resolution: New package function messageRenderer(ct, moderator, userType, stored, refs) with call nil, per item.
status: addressed
---
