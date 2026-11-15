---
id: RR-ZCV7M77
type: review-response
title: List bulk branch would clear the selection
finding: executeAction clears the selection and removes rows optimistically, which is wrong for a run that writes nothing.
severity: minor
resolution: 'Plan: the batch branch keeps the selection and the rows.'
status: addressed
---
