---
id: RR-P7TDGCJ
type: review-response
title: 'Design: Tasks candidate run read by subject id over-reads'
finding: Reading candidate runs by subject id returns all runs on those subjects and is cut at the page limit.
severity: significant
resolution: 'Plan updated: flowrun.Query gains IDs (run ids, at most MaxPage); Tasks reads exactly the candidate runs in one call, pinned by a budget test.'
status: addressed
---
