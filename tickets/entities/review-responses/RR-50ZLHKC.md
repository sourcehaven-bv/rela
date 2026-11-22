---
id: RR-50ZLHKC
type: review-response
title: "Boot meta was written for every static file"
finding: "bootShell ran before the check for whether the request gets the shell."
severity: minor
resolution: "Addressed: it runs only for a request that gets the shell, and the doc comment says so."
status: addressed
---
