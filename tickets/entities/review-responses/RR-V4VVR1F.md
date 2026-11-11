---
id: RR-V4VVR1F
type: review-response
title: Silent baseline conflicts with persisted seen set
finding: A new lock holder would silently swallow entries that arrived during hand-off.
severity: minor
resolution: Silent baseline only when no stored set exists or the setting is turned on; seen set pruned to ids in the latest read.
status: addressed
---
