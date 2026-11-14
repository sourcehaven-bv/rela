---
id: RR-Q2WOYS3
type: review-response
title: The archplan baseline edit was not staged
finding: .archplan/baseline.json showed only in the unstaged diff, so CI would see the moved findings as new.
severity: significant
resolution: The baseline is staged with the commit; archplan check reports 0 worse metrics and no new finding beyond the base.
status: addressed
---
