---
id: RR-01FG2ZL
type: review-response
title: "app.logo may name another host"
finding: "The logo on the workspace pages can load from an https host the operator chose."
severity: nit
resolution: "Won't fix: config is operator-authored and not a secret; the link rule allows only same-site paths and https."
reason: "Operator-authored config; same-site paths and https only."
status: wont-fix
---
