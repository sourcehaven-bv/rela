---
id: RR-RMIYWH3
type: review-response
title: No hidden-member test for CalDAV by-id read
finding: The dynamic collection by-id read has no test with a hidden member.
severity: minor
resolution: TestDynamicCollections_HiddenMemberIsAbsent lists a dynamic collection under a declarative ACL with an unreadable member; TestDynamicCollections_GoneDriverIsNotFound pins the gone-driver 404.
reason: ByIDs row gating is pinned by the existing TestScopedHeaders ByIDs tests; listByIDs only calls scopedEntities with ByIDs.
status: addressed
---
