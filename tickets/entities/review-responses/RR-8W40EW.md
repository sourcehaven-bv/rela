---
id: RR-8W40EW
type: review-response
title: Degrade-to-entity-level path silently leaked (no provenance = no redaction)
finding: search.Visible.SearchVisibleFields silently skipped field redaction when the wrapped searcher didn't satisfy the (unexported) provenance interface — the 'enforced at composition root' claim was false, so any decorator (metrics/caching/tracing) between the Service and NewVisible would reopen the oracle with no error.
severity: significant
resolution: 'SearchVisibleFields now FAILS CLOSED: when a hidden func is supplied but prov==nil it yields search.ErrScope instead of returning un-redacted hits. Added regression test TestSearchVisibleFields_FailsClosedWithoutProvenance wiring a non-provenance decorator and asserting ErrScope + zero hits. searchVisibleHits engages the field path only under a policy-backed resolver so the guard can''t misfire on NopACL.'
status: addressed
---
