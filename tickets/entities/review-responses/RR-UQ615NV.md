---
id: RR-UQ615NV
type: review-response
title: mark could drop the job attempt
finding: mark rebuilds Pending from scratch and would drop Job if called after the job started.
severity: minor
resolution: 'Plan: mark refuses to drop a job attempt; backoff copies it; a test pins backoff keeps it.'
status: addressed
---
