---
id: RR-F361VR
type: review-response
title: TakeChanged has one consumer and expiry depends on posts
finding: Ephemeral expiry produces no frame without posts; MCP watch would steal keys. Per-consumer change sets; sweep every tick.
severity: significant
resolution: 'Plan updated: .ignored/t3-plan.md section 6 (Design review folded in) redesigns this point; implementation follows that section.'
status: addressed
---
