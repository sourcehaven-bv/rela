---
id: RR-VN5OF02
type: review-response
title: 'Code: ellipsis had no effect and nowrap leaked'
finding: text-overflow on the inline-flex option did nothing; white-space nowrap applied to every segmented control.
severity: minor
resolution: Truncation moved to the label span in link mode with a title; nowrap is scoped to link mode.
status: addressed
---
