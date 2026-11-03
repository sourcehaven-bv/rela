---
id: RR-Y1ITY9
type: review-response
title: Version tag writes had no read gate
finding: TagVersion and UntagVersion and TagCurrent without a token read raw and authorized; reachable from Lua
severity: significant
resolution: prepare now calls requireReadableRow and answers newEntityNotFound; TestVersionTags_HiddenRowReadsAsMissing
status: addressed
---
