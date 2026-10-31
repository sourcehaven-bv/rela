---
id: RR-IYJA6KU
type: review-response
title: A failed re-load must not fail the whole validation section
finding: 'Design review: a ResolveHeadersErr failure in analyzeValidations should add one error issue for the rule and continue, as a CheckRuleFull failure should. Log only when ctx.Err() is nil. Specify what the orphan check does on the same failure.'
severity: significant
resolution: 'violationIssues returns its error; analyzeValidations adds ruleNotChecked (rule name, fixed message, slog.Warn unless ctx is done) and continues, for both CheckRuleFull and re-load errors. analyzeOrphans uses checkFailed, the cardinality pattern. Tests: TestAnalyzeValidations_ResolveFaultNamesTheRule, TestAnalyzeOrphans_ResolveFaultReportsOneIssue.'
status: addressed
---
