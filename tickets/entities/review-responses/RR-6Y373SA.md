---
id: RR-6Y373SA
type: review-response
title: SPA refuses files by browser-reported type
finding: The composer refused files whose browser MIME did not match accept, which disagrees with the server's content sniffing.
severity: minor
resolution: The client keeps count and size checks only; accept is the picker filter and the server judges content.
status: addressed
---
