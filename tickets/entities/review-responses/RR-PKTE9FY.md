---
id: RR-PKTE9FY
type: review-response
title: Capture without run_as reads as the shared scheduler identity
finding: A stored figure is a cached aggregate readable by every reader of the owner.
severity: significant
resolution: run_as is required on capture tasks; the guide names the stored figure as a deliberate cross-principal cache.
status: addressed
---
