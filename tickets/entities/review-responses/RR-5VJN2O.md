---
id: RR-5VJN2O
type: review-response
title: Ephemeral ring can hold about 4 GiB
finding: 4096 rings x 64 entries x 16 KiB. Cap ephemeral props (1 KiB), add a global byte budget and per-identity live-entry cap, refuse with 429 rather than evicting others.
severity: significant
resolution: 'Plan updated: .ignored/t3-plan.md section 6 (Design review folded in) redesigns this point; implementation follows that section.'
status: addressed
---
