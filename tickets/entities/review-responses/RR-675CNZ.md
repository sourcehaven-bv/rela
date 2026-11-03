---
id: RR-675CNZ
type: review-response
title: Budget test must spread entries over distinct anchors, pairs and the creator branch
finding: 'AC 11 says "equal at 10 and 50 entries, half the anchors hidden". Fix: state that the N entries sit on N distinct anchors across at least two waiting pairs, plus creator-branch channels, under a permission read role conferred by a relation. Otherwise a per-anchor HoldsPermissionFor walk or a per-pair loop does not change the count between 10 and 50 and the test cannot fail.'
severity: minor
resolution: AC 11 now puts N entries on N distinct anchors over at least two pairs plus creator-branch channels, under a relation-conferred permission read role, with half the anchors hidden.
status: addressed
---
