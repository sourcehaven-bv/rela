---
id: RR-XDCLMHW
type: review-response
title: Stored-hash-equals-view-hash not tested
finding: Nothing pinned that SumEntity hex equals HashEntity or that an unredacted view equals the stored hash.
severity: minor
resolution: Tests check hex(SumEntity) == HashEntity and the same for relations; that EntityElement of an unredacted row carries HashEntity; and that a hidden set property and an absent one give the same view hash.
status: addressed
---
