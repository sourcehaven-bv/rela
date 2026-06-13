---
id: RR-BDYTP5
type: review-response
title: /_search has no limit param; 'truncate to requested limit' undefined and unbounded post-ACL scan is a DoS amplifier
finding: handleV1Search serializes all results (Meta.PerPage = len(data)); there is no limit query param, so step 2's 'truncates to requested limit' has no defined source for the /_search consumer. If unbounded, a broad query forces an uncapped candidate scan + per-type MatchingIDs over the whole corpus — 'MatchingIDs is cheap' per call does not bound an unbounded candidate set.
severity: significant
resolution: 'Plan rev 2: /_search keeps no user limit param; the ''requested limit'' is the existing maxFreeTextSearchResults=1000, passed as Query.Limit to SearchVisible and applied post-visibility. DoS bound: candidate set capped by backend (bleve 10k floor / linear corpus / pgstore SQL LIMIT); MatchingIDs batched per type over the bounded candidate set. Non-free-text queries stay unbounded (matches today''s list-endpoint semantics), now gated.'
status: addressed
---
