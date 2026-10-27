---
id: RR-8S7OYK
type: review-response
title: Stale stored values under a roll-up name
finding: Frontmatter may carry a stored value under a roll-up name, which surfaces raw and blocks raw read-modify-write.
severity: significant
resolution: Responses drop any stored key named like a roll-up before merging. entitymanager refuses a roll-up key only when it differs from the stored value, as for computed properties.
status: addressed
---
