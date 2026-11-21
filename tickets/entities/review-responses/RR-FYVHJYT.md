---
id: RR-FYVHJYT
type: review-response
title: Webhook prefix duplicated across Go and TypeScript
finding: 'changeText.ts hard-codes webhook: while Go has idpWebhookUserPrefix; the two can drift.'
severity: nit
resolution: The TypeScript comment names the Go constant and its file.
status: addressed
---
