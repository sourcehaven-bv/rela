---
id: RR-D0Z4UXQ
type: review-response
title: The batch re-load must check Served()
finding: 'Design review: ResolveHeadersErr returns a ResolvedHeader for refs that exist but are not served. Using the header without Served() would report a hidden entity.'
severity: significant
resolution: Both the orphan and validation loops skip a ref that is missing or not Served(). TestAnalyzeValidations_HiddenViolatorTakesNoSlot covers a hidden violator.
status: addressed
---
