---
id: RR-3WEG3X
type: review-response
title: Other GraphQuery field enumerations
finding: pgstore/visiblesearch.go:326-341 reads only HasInbound/HasOutbound/Any and would ignore Related; pgstore prefix comment and GraphQuery type doc become stale.
severity: minor
resolution: 'Plan: buildVisibilityDisjunction errors on non-empty Related; name the rel%d prefix; update the comments and type doc.'
status: addressed
---
