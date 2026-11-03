---
id: RR-YW6XW4
type: review-response
title: len(moved) > 0 in the topic round condition is redundant
finding: Every moved key is also a changed key, so len(keys) > 0 already holds whenever moved is non-empty (live.go:687-688). Adding the term suggests that a moved key can arrive without a changed key. Either drop it, or keep it with a comment that it is defensive.
severity: nit
resolution: 'Adopted: the term is dropped and step 4 says why the existing len(keys) > 0 already covers moved keys.'
status: addressed
---
