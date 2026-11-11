---
id: RR-G7A444D
type: review-response
title: Seen-set write while focused is undocumented
finding: The seen set is written while a tab has focus, which silently consumes notifications; no comment said this is intended.
severity: nit
resolution: Comment at the saveSeen call explains that what is on screen counts as seen.
status: addressed
---
