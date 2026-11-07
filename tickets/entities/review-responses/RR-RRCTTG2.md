---
id: RR-RRCTTG2
type: review-response
title: per_subject many has no per-subject cap
finding: A buggy start trigger can create unbounded runs on one subject on pg and sqlite.
severity: minor
resolution: MaxRunsPerSubject per (flow, subject) checked in Create under a per-subject lock.
status: addressed
---
