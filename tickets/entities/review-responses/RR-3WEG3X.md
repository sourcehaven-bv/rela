---
id: RR-3WEG3X
type: review-response
title: Other GraphQuery field enumerations
finding: pgstore/visiblesearch.go:326-341 reads only HasInbound/HasOutbound/Any and would ignore Related; pgstore prefix comment and GraphQuery type doc become stale.
severity: minor
resolution: buildVisibilityDisjunction renders each Related entry with a v%d_rel%d prefix rather than refusing it; the pgstore prefix comment and the GraphQuery type doc name Related.
status: addressed
---
