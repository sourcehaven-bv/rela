---
id: RR-NOKSKF
type: review-response
title: List identity keys are guesswork
finding: Navigation items (- entities:, - label:, - group:) have no stable key; editing a label turned an item into delete+add and lost its comments.
severity: significant
resolution: 'Plan changed: GET tags each mapping inside a list with its original index ($i); the merge matches on it first (base_version pins the document), then identity, then position.'
status: addressed
---
