---
id: RR-61N2AY
type: review-response
title: Name grammar accepts Windows reserved names
finding: con, nul and similar pass Name but fail ValidateKey.
severity: minor
resolution: 'Plan R11: refuse them in NewName.'
status: addressed
---
