---
id: RR-W9BOQP3
type: review-response
title: serve.go comment width and wait order differ from the other hosts
finding: The startScheduler comment ran past the wrap width, and it waited on flows before the realtime worker, unlike the desktop and tenant hosts.
severity: nit
resolution: Rewrapped the comment and made the wait order match the other hosts.
status: addressed
---
