---
id: RR-7EKSAZV
type: review-response
title: False conflict after 'their body already holds our edit' while typing
finding: When a 412 refetch already held the edit, nothing was written, so neither the base nor contentRebase moved; typing made meanwhile was merged against the old base and reported as a conflict.
severity: significant
resolution: resolveConflicts returns the absorbed body; mergeServerResponse sets contentRebase onto it. Unit test 'rebases typing onto their body when it already held the save' fails without it.
status: addressed
---
