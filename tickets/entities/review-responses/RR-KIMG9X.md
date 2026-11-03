---
id: RR-KIMG9X
type: review-response
title: List budget test never exercises the creator-role branch
finding: 'internal/realtime/waiting_test.go:379-407. The reader alice owns TKT-1, and waitingView stops evaluating once onMe is true (waiting.go:64), so with waiting_for: [lead, starter] the per-row creator closure (sameAuthor + CeilingAdmitsUpdate, waiting.go:116-117) never runs. AC18 says the budget must cover a named role and a creator role. A per-row lookup added to the creator path would pass this test. Fix: order the list as [starter, lead], or measure with a reader who is not the owner, so every row evaluates the creator branch.'
severity: minor
resolution: TestListAnchor_WaitingForBudget now reads as a principal who is neither holder nor creator so the creator branch runs under the budget
status: addressed
---
