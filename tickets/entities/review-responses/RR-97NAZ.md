---
id: RR-97NAZ
type: review-response
title: Prev/Next preserving return_to not stated
finding: 'useScopeNavigation.navigateScope pushes with `query: route.query`, preserving return_to by accident. Plan''s ''keep scope-nav and return_to independent'' framing could invite a future commit that strips return_to from prev/next pushes. State explicitly in the plan + unit test: ''Prev/Next preserves return_to on the URL so Back keeps pointing at the original source across in-list navigation.'''
severity: minor
resolution: AC3 amended to state 'Prev/Next preserve return_to in the URL'. Unit test added to useScopeNavigation.test.ts asserting navigateScope preserves both from and return_to on the push call.
status: addressed
---
