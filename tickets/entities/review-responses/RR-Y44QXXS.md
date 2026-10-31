---
id: RR-Y44QXXS
type: review-response
title: refreshView never sets viewWorld
finding: A world switch could land through refreshView, leaving viewWorld on the old world so content links opened in it.
severity: significant
resolution: Every landing goes through landView, which sets viewData and viewWorld together.
status: addressed
---
