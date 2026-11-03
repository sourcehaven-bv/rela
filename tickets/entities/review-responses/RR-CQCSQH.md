---
id: RR-CQCSQH
type: review-response
title: Lua hid target lookup faults as an empty page
finding: Any GetAddress error became an empty page, so a denied reader or cancelled context looked like no backlinks.
severity: nit
resolution: Only store.ErrNotFound gives an empty page; any other error raises. Docs updated.
status: addressed
---
