---
id: RR-RTRYNJ
type: review-response
title: Loops with more than 8 drawn calls per iteration no longer fold
finding: children flattened same-package calls before folding, so a helper making 9+ cross-package calls per item drew every call unfolded.
severity: significant
resolution: Restored folding by child call (any body size) before the periodic fold; TestFoldLargeHelper.
status: addressed
---
