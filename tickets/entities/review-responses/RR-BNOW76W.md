---
id: RR-BNOW76W
type: review-response
title: Risk statement about dates was wrong
finding: 'Design review: NilOrderedIsFalse applies to Int and String only, so unset dates against today() still error. The real change is string/integer comparisons on unset values inside unlowered conditions.'
severity: minor
resolution: Plan corrected; TestEvaluate_UnsetOrderedComparisonIsFalse pins the new behavior and GUIDE-desktop states the rule.
status: addressed
---
