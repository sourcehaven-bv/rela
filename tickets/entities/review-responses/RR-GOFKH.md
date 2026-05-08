---
id: RR-GOFKH
type: review-response
title: Two flatteners share 90% of code and risk drift
finding: renderInlineNode and flattenInlineNode differ in three boolean toggles (preserve link/emphasis wrappers, breaks-as-newline, image-as-markdown). Could fold into one configurable function.
severity: minor
reason: 'Code clarity tradeoff: separate functions today are easier to read; consolidation can come later if a new policy emerges. Defaults are documented.'
status: deferred
---
