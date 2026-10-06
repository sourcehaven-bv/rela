---
id: RR-DYURK3
type: review-response
title: Unclassified face='' literals in search and graph total
finding: Section 3's audit omits pg search.go and visiblesearch.go default branches; sqlite buildGraphTotalSQL; and pg/sqlite GetRelation and purge from_face=''.
severity: minor
resolution: 'Amendment A8: search and graph-total are the InWorld(zero) fast path and stay; relation and purge literals move to the key''s FromFace in PR 7.'
status: addressed
---

## Finding

Section 3's audit omits pg search.go and visiblesearch.go default branches;
sqlite buildGraphTotalSQL; and pg/sqlite GetRelation and purge from_face=''.

Design: `.ignored/stage2-design.md` section 11.
