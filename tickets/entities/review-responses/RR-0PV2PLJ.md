---
id: RR-0PV2PLJ
type: review-response
title: Test gaps in the plan
finding: 'Design review: add budget tests with store and ACL graph counted, under both redactors, with chunk scaling; cap boundaries 100/101/across rules/miss as the 101st; a faced violator in a world; a script error after a full section; parity.'
severity: significant
resolution: Added TestQueryBudget_AnalyzeIsSizeIndependent, TestQueryBudget_AnalyzeWithFieldRulesIsSizeIndependent (fails at 122 vs 522 reads without the page prime), TestQueryBudget_PushdownOnPrimedCtxAddsNoPrimeReads, TestRedactingSeq_* (paging, error, early stop, nil), the cap and fault tests, the hidden-violator and hidden-title tests, and TestAnalyze_FacedValidationViolators. Parity was checked on the atlas copy. A script error after a full section is cut by capIssues, the subject of the first finding.
status: addressed
---
