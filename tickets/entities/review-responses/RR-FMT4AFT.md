---
id: RR-FMT4AFT
type: review-response
title: SSE stream keeps an expired grant
finding: freshReadGate re-derives only on role-relation events; expiry emits none.
severity: minor
resolution: 'Plan: refresh the gate when the bound day changes.'
status: addressed
---
