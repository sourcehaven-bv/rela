---
id: RR-EB9KR7
type: review-response
title: Drill fast path is lost for faced hierarchies
finding: The subtree closure walks every face's content edges, so a faced parent almost always declines to the full build.
severity: minor
reason: 'Superseded by the security fix: the drill now declines for every faced source by design. Restoring the fast path needs a world on the closure query, which is TKT-KQXVF7 scope; recorded for it in the BUG-BZQQDP resolution.'
status: deferred
---
