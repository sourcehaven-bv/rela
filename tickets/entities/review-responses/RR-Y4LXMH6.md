---
id: RR-Y4LXMH6
type: review-response
title: Tests do not pin that list reads skip the run lookup
finding: No test showed that a list page, which only redacts, does no run lookup; the fake run store also ignored an empty SubjectIDs.
severity: minor
resolution: TestQueryBudget_ListDoesNoFlowRunLookup pins zero lookups for a list page; the fake treats empty SubjectIDs as no narrowing.
status: addressed
---
