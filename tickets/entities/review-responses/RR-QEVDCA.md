---
id: RR-QEVDCA
type: review-response
title: Face delete block answer reveals a hidden face
finding: Block on family channels is checked only when the stored last face goes, so success vs 409 tells a caller whether an unreadable face exists. Check block when no caller-readable face would remain; apply detach when the stored last face goes.
severity: significant
resolution: 'Plan updated: .ignored/t3-plan.md section 6 (Design review folded in) redesigns this point; implementation follows that section.'
status: addressed
---
