---
id: RR-QYKY2XH
type: review-response
title: No rule for mixing entity and relation addresses
finding: Address forms are disjoint only through the id and type grammars. The plan did not say whether a range may mix kinds.
severity: minor
resolution: 'Documented: the ordering key is (Kind and Address) and a range never spans kinds. The relation address is RelationKey.String() with the tail face.'
status: addressed
---
