---
id: RR-TMDW8OD
type: review-response
title: not_run retried without cap
finding: A range that always exceeds the server deadline answers not_run forever until the round-trip cap. Count per span; split after one; fail after a few.
severity: significant
resolution: not_run is counted per span and capped by Options.MaxNotRun (default 4). The first is a plain retry; then a list halves its limit and a fingerprint splits. Only the first not_run of an answer counts.
status: addressed
---
