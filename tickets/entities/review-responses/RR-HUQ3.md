---
id: RR-HUQ3
type: review-response
title: 'Offline / failed dry-run: fail-open UI is correct, but commit must still gate'
finding: 'Unspecified: what does the create form do if the dry-run request fails (network error, server down, 500)? Since dry-run is advisory, the form should fail-open (render fields normally, don''t block the user) — the commit gate is the real boundary, so a failed hint cannot create a security hole, only a worse UX (user finds out at save). Document this explicitly so an implementer doesn''t ''fail closed'' and brick the create form when the affordance check is unavailable. Test: dry-run 500 -> form still usable -> commit still 403s a denied field.'
severity: minor
resolution: 'Plan: dry-run failure (network/500) -> form fails OPEN (renders normally, no block); the commit gate is the boundary so a missing hint is only a UX regression, never a security hole. Test: dry-run 500 -> form usable -> commit still 403s a denied field.'
status: addressed
---
