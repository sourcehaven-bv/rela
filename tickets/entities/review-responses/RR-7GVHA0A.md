---
id: RR-7GVHA0A
type: review-response
title: Sparse match reads one OFFSET window after another to the end of the type
finding: firstNByTypes doubled windows up to 500 and kept going to the end of the type when the Go filter kept few rows (only OpEqual is pushed). At 13.6k rows that is about 31 statements with growing OFFSETs on pg, and on fsstore and memstore every window is a full scan.
severity: significant
resolution: Windows now grow fourfold and after three windows the rest of the type is read in one go, so a sparse match costs at most one read more than an unpaged read. TestFirstN_SparseMatchReadsBoundedWindows pins four reads for a 500-row type with no match.
status: addressed
---
