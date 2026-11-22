---
id: RR-8452XIH
type: review-response
title: "The read-only flag went stale"
finding: "The flag was read once at boot, so a state an operator changed later needed a reload."
severity: significant
resolution: "Addressed: it is read again after the schema loads and whenever the tab becomes visible."
status: addressed
---
