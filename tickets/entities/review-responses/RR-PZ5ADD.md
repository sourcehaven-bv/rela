---
id: RR-PZ5ADD
type: review-response
title: History rule depends on the snapshot face
finding: 'The history decision is sound (history roles are a subset of live roles and the union is monotone), but a global read: [type@face] grant must contribute nothing on another face''s snapshot. Pin it with a test, and update the WithHistoricalSubject godoc and the history invariant text in the security guide.'
severity: minor
resolution: TestRoleUnion_History covers published, draft and unnamed-face snapshots; WithHistoricalSubject godoc and the history section of GUIDE-acl-security describe the face dependency.
status: addressed
---
