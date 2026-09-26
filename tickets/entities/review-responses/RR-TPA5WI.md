---
id: RR-TPA5WI
type: review-response
title: Gated searcher returned unredacted index titles
finding: GatedReadBundle.Searcher yielded Hit.Title from the index; only a godoc warned consumers not to show it.
severity: significant
resolution: gatedSearcher now clears Hit.Title on every hit. Pinned by TestGatedReads_SearcherHitShape.
status: addressed
---
