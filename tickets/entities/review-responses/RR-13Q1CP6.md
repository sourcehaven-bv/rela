---
id: RR-13Q1CP6
type: review-response
title: Shared base reuse conflicts with a per-case clock
finding: prepare stores opts on SharedBase; SharedBase.Assemble takes no per-call options.
severity: significant
resolution: Each case runs a full prepare+assemble with its own options; no shared mutable clock. Config parse cost measured on tickets/ (risk listed).
status: addressed
---
