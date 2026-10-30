---
id: RR-7WATKO4
type: review-response
title: AbortController aborts nothing
finding: fetchView takes no signal, so the session's AbortController has no effect.
severity: nit
resolution: fetchView takes an optional signal and the session passes it.
status: addressed
---
