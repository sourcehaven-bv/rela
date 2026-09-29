---
id: RR-TYJON4
type: review-response
title: collectEdgeWarnings reads each peer separately
finding: 'collectEdgeWarnings calls readableType per edge: one header read for the stored type and one for Family plus a gate call. A relationships body with many edges costs about three queries per edge. There is no storetest.Counting budget test.'
severity: significant
reason: 'The per-edge read predates this PR: the old code did one ungated GetEntity per edge (which also leaked a hidden peer''s type). This PR gates it and keeps the shape. Resolver.ResolveHeaders (PR 5b) batches the family check but returns no stored type for an unserved face and answers gate errors as misses. Using it here would undo RR-OGJ0NW. A typed batch read is the follow-up. The write path calls the manager once per edge so it stays linear either way.'
status: deferred
---
