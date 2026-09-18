---
id: RR-BCUHUU
type: review-response
title: Doc rhetoric, a falsifiable claim, and a third stale copy
finding: 'Three smaller items. (1) The StateFinding.Code doc called bare-row-on-faced-type the headless-family check''s MIRROR, which reads well and informs poorly — headless-family fired on any family lacking a bare row, this fires on a bare row existing on a faced type; two predicates over overlapping data, not inverses. (2) The Detail string claimed ''no world can reach it'', which the perf fixture''s own schema comment falsifies: a world with otherwise: default does reach such rows via its fallback. (3) docs/metamodel.md still described analyze states in pre-fix terms — the third of three places this is documented, and the commit updated two.'
severity: minor
resolution: '(1) Replaced the mirror claim with the fact: the removed check required a bare row, this one reports one nothing can reach; different predicates, not inverses. (2) Narrowed to ''no world''s chain names it'', which is accurate — the chain lists declared faces, never the bare id, and the otherwise: default fallback is not the chain. Separately found the same class of error in my own new unknown-entity-type sentence, which claimed no world could reach an undefined type; probed ResolveWorldPrimes and it returns Via:0 (unscoped, rule 1), so the row IS served. Rewritten. (3) Updated docs-project/entities/guides/GUIDE-metamodel.md (the source, not the generated output) with a table of all three finding codes.'
status: addressed
---

Item 2 is the one worth remembering. The reviewer caught one falsifiable
absolute; checking it turned up a second one I had just written into the new
`unknown-entity-type` sentence, where I asserted no world could reach the row
without probing whether that was true. It is not — an unscoped type falls to
resolution rule 1 and is served.

The pattern is the same as RR-Y0UN58 on the previous PR: an absolute claim about
world behaviour, written from reasoning rather than from a probe. Probing takes
about a minute.
