---
id: RR-6OMFHE
type: review-response
title: Dutch under-a-minute wording breaks the pattern
finding: 'frontend/packages/rela-components/src/composables/messages.nl.ts:36-37 returns "minder dan een minuut", so the name reads "Agent run: Planning, minder dan een minuut". The other units read "al 3 uur". Fix: use "nog geen minuut" or "korter dan een minuut" so the sentence parses like the others.'
severity: nit
resolution: Dutch now reads nog geen minuut, matching the al 3 uur pattern; the tail helper takes the unit suffix.
status: addressed
---
