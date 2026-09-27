---
id: RR-48FBVK
type: review-response
title: MatchingIDs query must be built fresh; answers never cached across principals
finding: Copying rqr.Query risks one HasInbound overwriting the other; answers must live per call, not on the per-(cfg, meta) resolver. Add an interleaved two-principal test.
severity: nit
resolution: acl.TraversalQuery builds a fresh GraphQuery per traversal, placing the predicate by direction. Answers live in the Filter call only. TestQueryScopeTraversal_EndToEnd interleaves alice, bob and carol on one app.
status: addressed
---
