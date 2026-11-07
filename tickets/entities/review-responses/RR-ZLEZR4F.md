---
id: RR-ZLEZR4F
type: review-response
title: 'Security: bodies are not covered by HiddenField'
finding: The stored hash covers the body; body redaction does not exist but the TODO lists read paths to change.
severity: nit
resolution: Godoc states HiddenField covers properties only and the range read is added to the TODO(body-redaction) list.
status: addressed
---
