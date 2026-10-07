---
id: RR-5Y1BPF
type: review-response
title: Backend Open result not checked for nil
finding: A backend returning a nil Close or nil store would panic mid-copy.
severity: nit
resolution: run.open rejects an incomplete Opened. TestRun_RejectsIncompleteBackend.
status: addressed
---
