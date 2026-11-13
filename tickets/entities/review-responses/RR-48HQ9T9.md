---
id: RR-48HQ9T9
type: review-response
title: Header status failed is terminal but the run stays retriable
finding: flow.Status.Terminal treats failed as ended; flowsteps and closeOrphan would read the run as over.
severity: critical
resolution: 'Plan: the header status is left alone. The explanation reports the run as failed with the failure kind; the view and API show it.'
status: addressed
---
