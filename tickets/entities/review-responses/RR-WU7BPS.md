---
id: RR-WU7BPS
type: review-response
title: Label cap cascades into undefined-label errors
finding: Over MaxLabels, parsing returned before reading any label, so every assignment also reported an undefined label.
severity: nit
resolution: The first MaxLabels labels are still read; the label-count test asserts only the limit issue is reported.
status: addressed
---
