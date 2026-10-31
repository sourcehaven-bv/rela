---
id: RR-SS939U9
type: review-response
title: Nothing tests the early stop at one past the cap
finding: 'Code review: Removing room from violationIssues passes every test, because capIssues cuts the output to the same 100 issues.'
severity: significant
resolution: TestAnalyzeValidations_StopsReloadingAtCap and TestAnalyzeOrphans_StopsReloadingAtCap count the refs per re-load batch. Both fail with room removed.
status: addressed
---
