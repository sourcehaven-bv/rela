---
id: RR-D5Q9LQ
type: review-response
title: Undefined label could nil-dereference in subjects
finding: identifies looked up f.Labels[label].Role directly, which panics on a file with an undefined label that is still reported on.
severity: minor
resolution: Added roleOf with a nil guard. Pinned by TestBuildReport_UndefinedLabel.
status: addressed
---
