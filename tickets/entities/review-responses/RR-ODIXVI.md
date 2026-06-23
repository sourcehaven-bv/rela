---
id: RR-ODIXVI
type: review-response
title: AC6 (lift cap) and AC9 (NopACL byte-parity) are contradictory as written
finding: NopACL → all-AllowAll → uncapped search returns more than 1000 results where today's /_search returns ≤1000 for a large corpus, so the JSON-canonical regression cannot be byte-identical. Both ACs cannot be green simultaneously on a corpus >1000 matching entities; the plan must pick a cap model.
severity: significant
resolution: 'Plan rev 2 cap model: maxFreeTextSearchResults=1000 stays as the FINAL /_search result bound but moves from pre-ACL candidate cap to post-visibility truncation. NopACL/all-AllowAll: no-op filter + unchanged ranker → top-1000 byte-identical to today, AC9 holds. Restricted principals get the true top-1000 of their visible corpus. Contradiction dissolved.'
status: addressed
---
