---
id: RR-2N2NGMB
type: review-response
title: append aliasing in search_hydrate
finding: append(hits[:10], ...) overwrites hits[10].
severity: nit
resolution: Uses slices.Concat.
status: addressed
---
