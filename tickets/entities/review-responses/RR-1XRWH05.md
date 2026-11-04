---
id: RR-1XRWH05
type: review-response
title: Chat type that moves the session removes the composer
finding: docs/realtime.md allows a chat type that moves the session in one state (e2e Contained, channels.spec.ts:190); as a button type and the only pickable type it would leave no composer.
severity: significant
resolution: 'Plan 3: safeguard, when leaving out button types empties the picker, the picker lists every offered type (AC 5, edge cases).'
status: addressed
---
