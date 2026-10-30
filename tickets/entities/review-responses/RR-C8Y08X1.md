---
id: RR-C8Y08X1
type: review-response
title: New guards in comments and scope navigation untested
finding: The note fixture is not commentable and the test routes carry no ?from=, so the loadComments and loadScopeNav guards were never exercised.
severity: significant
resolution: Added EntityDetail.comments.switch.test.ts and two useScopeNavigation tests; each fails without its guard.
status: addressed
---
