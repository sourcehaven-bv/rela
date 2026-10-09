---
id: RR-GBM0DP
type: review-response
title: Automation script output was gated
finding: cascadeWrite is set only on DeleteEntity; script actions got m.gated(), which was the field-gated manager.
severity: significant
resolution: gated() drops fieldGate, so cascade dispatch is ungated; TestGated_DropsFieldGate pins it.
status: addressed
---
