---
id: RR-6VNZWR
type: review-response
title: Live hub belongs in its own package
finding: 'Architecture review: the hub depends only on http.Handler, principal and a feed, but lives in dataentry, where later edits can reach into App and the multi-instance forwarding seam would land.'
severity: minor
reason: Moved to follow-up TKT-H6LD1Y, to be done with the multi-instance work of FEAT-GWF8YH, which adds the forwarding seam that package exists for. The hub has no App pointer today, so the move stays mechanical.
status: deferred
---
