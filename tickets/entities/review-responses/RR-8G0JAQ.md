---
id: RR-8G0JAQ
type: review-response
title: Token and SPA details unspecified
finding: Base64 variant, exact length, seq <= head check, header vs query precedence, ignoring empty lastEventId, and which frame a resync sends were not written down.
severity: minor
resolution: 'All recorded in the plan: RawURLEncoding strict, 48 chars, seq <= head, header wins, empty lastEventId ignored, resync sends refresh instead of ready.'
status: addressed
---
