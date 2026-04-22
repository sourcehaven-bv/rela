---
id: RR-SG0LP
type: review-response
title: document-live-update tests should be ported as test.skip, not dropped
finding: Those tests encode intended behavior. Porting with test.skip preserved + TODO costs nothing and keeps the intent recorded.
severity: minor
resolution: Ported as e2e/tests/document-live-update.spec.ts with three test.skip stubs and comments describing intended behaviour + unskip conditions.
status: addressed
---
