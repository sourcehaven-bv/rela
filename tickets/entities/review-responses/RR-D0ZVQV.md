---
id: RR-D0ZVQV
type: review-response
title: Casts must be total so hidden rows cannot fail a page
finding: A cast on a malformed value can raise in SQL; the planner does not order the gate before it. Overflow handling differs per backend.
severity: critical
resolution: Every extraction is a nested CASE on the JSON type (pg jsonb_typeof, sqlite json_type) before any cast; aggregates only see rows that passed the gate. Sum overflow yields no value on every backend (generic checked add, pg numeric then int64 parse, sqlite split hi/lo sum combined in Go). Conformance test with malformed and overflowing values.
status: addressed
---
