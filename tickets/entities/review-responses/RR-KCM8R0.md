---
id: RR-KCM8R0
type: review-response
title: TestNavPermission_FilterIsPresentationOnly was theatre
finding: 'The test guarding the feature''s central claim couldn''t fail from a change to the feature. It asserted handleV1ListEntities returns 200 for bob — but bob''s role grants Read on ticket, so that 200 holds for unrelated reasons, and handleV1ListEntities has no knowledge of navigation config at all (no import, no lookup, no shared state). It asserted that two unconnected subsystems are unconnected. The reviewer mutation-tested it: with permitsNavEntry hardwired to return true, only the precondition line failed (a duplicate of NonHolderFiltered), never the advertised assertion. It also never checked the response body, so it would not have noticed the target silently returning zero rows.'
severity: significant
resolution: 'Rewritten to test the two cases where menu and data could actually diverge, in both directions: (a) bob is denied the permission but permitted the data — entry hidden, list returns rows (hiding gates nothing); (b) carol holds the permission but is denied the data — entry shown, list returns empty (data-gating gates no menu). The rows assertion is the load-bearing half. Mutation-verified: with the filter disabled, TestNavPermission_FilterIsPresentationOnly now fails on the assertion rather than the precondition. Also added TestNavFilterStaysPresentational, a grep test forbidding permitsNavEntry calls outside views_handler.go — the reviewer''s suggestion, following the translateVerb precedent in lint_test.go. Worth it because a prose rule already failed to prevent exactly this drift once (TKT-M1AX6P).'
status: addressed
---
