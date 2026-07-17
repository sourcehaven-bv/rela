---
id: RR-WY3CO0
type: review-response
title: emitIncomingDiff can silently drop a picked id missing from candidates
finding: 'In emitIncomingDiff, a newly-added id only enters currentEntries if found in candidates; buildRelationsPatch builds data from entries (added is decorative), so a picked id missing from candidates would silently vanish from the POST — the same bug class in a different door. Latent only: every selectable id today enters via selectEntity whose source is candidates (or handleEntityCreated which pushes before selecting).'
severity: nit
reason: Latent fragility, not a live bug and not introduced by this change. Every id that can be selected is guaranteed to be in candidates at emit time (dropdown items come from candidates; inline-created targets are pushed to candidates before selection). Hardening emitIncomingDiff with a fallback/warn is a reasonable follow-up but out of scope for this bug fix; deferring to keep the fix minimal and focused.
status: wont-fix
---
