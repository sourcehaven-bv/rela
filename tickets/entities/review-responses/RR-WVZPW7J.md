---
id: RR-WVZPW7J
type: review-response
title: Candidate scan reads full entities with bodies
finding: collectCandidates listed candidates with ListEntities, loading bodies, against the collection-reads rule.
severity: significant
resolution: Candidates come from scopedHeaders (the list path's gated header read) and each page is redacted once. Budget test added with storetest.Counting.
status: addressed
---
