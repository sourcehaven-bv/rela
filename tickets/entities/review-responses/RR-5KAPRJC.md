---
id: RR-5KAPRJC
type: review-response
title: sqlitestore header page errors say list entities page
finding: queryEntityPage wrapped every error as list entities page so a header page failure named the wrong read.
severity: nit
resolution: queryEntityPage takes an op label; the header page reports list entity headers page.
status: addressed
---
