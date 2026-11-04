---
id: RR-7N09FZQ
type: review-response
title: 'Inert render: input on computed, roll-up and external_ref columns'
finding: These properties are never writable, so the key could do nothing.
severity: nit
resolution: Config load refuses them (same check as file properties).
status: addressed
---
