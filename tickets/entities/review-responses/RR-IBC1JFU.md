---
id: RR-IBC1JFU
type: review-response
title: Operator-forensic fields on the per-entity view
finding: AssertedBy, Flow, Run and By are not needed by entity readers.
severity: minor
resolution: 'Plan: /_changes carries time, op, subject, principal user and tool, allowlisted summary and trigger kind only; /_audit keeps the full record.'
status: addressed
---

AssertedBy, Flow, Run and By are not needed by entity readers.
