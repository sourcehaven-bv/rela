---
id: RR-PQ52P72
type: review-response
title: Start never appears after a failed runs load
finding: 'FlowRunsPanel.vue: a load error other than a cancel or refusal left loaded false, so Start never showed.'
severity: significant
resolution: 'Addressed: the generic error branch sets loaded, since the server refuses a second run itself. Test: still offers Start when the runs fail to load.'
status: addressed
---
