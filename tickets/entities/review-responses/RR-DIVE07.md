---
id: RR-DIVE07
type: review-response
title: Presence join can outlive its subscription
finding: baseline and round commits write through a stale liveSub so a join can land after the sub was dropped or the connection closed leaving a phantom viewer
severity: significant
resolution: 'Design updated before implementation: .ignored/phase2-plan.md section Presence security design review folded in; the implementation must carry the listed test.'
status: addressed
---
