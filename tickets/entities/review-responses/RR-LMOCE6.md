---
id: RR-LMOCE6
type: review-response
title: worldText substitutes with four split/join passes; empty vs undefined var behaviour is undecided
finding: frontend/src/utils/worldText.ts allocates five arrays per call, per badge, per row. One regex pass is cheaper and makes the unknown-placeholder rule structural. An undefined var leaves {face} written while an empty var substitutes to nothing; WorldBadge.test.ts works around this instead of the code deciding it.
severity: nit
resolution: 'worldText is one regex pass over the allowlist. Decided and documented: an undefined var leaves the placeholder as written (the surface has no such fact), an empty var substitutes to nothing (the fact is empty); a substituted value is never re-scanned. Three new unit tests pin each.'
status: addressed
---
