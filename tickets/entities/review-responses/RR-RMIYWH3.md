---
id: RR-RMIYWH3
type: review-response
title: No hidden-member test for CalDAV by-id read
finding: The dynamic collection by-id read has no test with a hidden member.
severity: minor
reason: ByIDs row gating is pinned by the existing TestScopedHeaders ByIDs tests; listByIDs only calls scopedEntities with ByIDs.
status: wont-fix
---
