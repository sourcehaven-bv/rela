---
finding: ViewNames() iterates over a map which has non-deterministic order. This affects testability and could cause confusing output.
id: RR-ygyq
resolution: Added SortedViewNames() method that returns view names in alphabetical order. ViewNames() now delegates to SortedViewNames() for consistent ordering.
severity: significant
status: addressed
title: ViewNames returns unpredictable order
type: review-response
---
