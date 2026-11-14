---
id: RR-5H3ME4L
type: review-response
title: Total of relate and related limits is unbounded
finding: Each limit is capped but an action with many entries could send thousands of items.
severity: minor
resolution: Config load refuses limits adding up to more than 400 (MaxLLMTotalItems); tested.
status: addressed
---
