---
id: RR-I4FN1T
type: review-response
title: LookupByEntity is map-order nondeterministic, flipping the href between polls
finding: LookupByEntity iterates a Go map and returns the first match, which Go randomizes. Two aliases for one entity is trivially reachable (PUT the same rela-shaped UID at two hrefs; both 201), after which objectFor picks a different winner per call. Reviewer measured winners across 200 calls spread over 5 hrefs, and observed the served href changing between REPORTs. A CalDAV client reads a changed href as delete-plus-create, so the to-do vanishes and reappears on random sync cycles. Worse, the ctag hashes only ETags and never hrefs, so it stays constant across the flip - a polling client can skip the resync entirely and sit on a dangling href.
severity: significant
resolution: 'Two-part fix. Put now evicts any other href in the same collection pointing at the same entity, so the ambiguous state cannot arise (newest wins - that is the href the client just used). LookupByEntity additionally picks the lowest href deterministically, as belt-and-braces for tables written before the eviction existed. TestOneEntityOneResource pins both: the superseded href is gone, 50 consecutive LookupByEntity calls return the same href, and an unrelated entity in the same collection is untouched. TestStoredFormIsStable was using one entity id across three hrefs - a state now forbidden - so it was given distinct ids; its intent (sort stability of the on-disk form) is unchanged.'
status: addressed
---
