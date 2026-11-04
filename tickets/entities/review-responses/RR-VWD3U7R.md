---
id: RR-VWD3U7R
type: review-response
title: forms.spec create check listens on response
finding: A create POST whose response is late would be missed; the assertion is about the request.
severity: minor
resolution: The listener uses the request event.
status: addressed
---
