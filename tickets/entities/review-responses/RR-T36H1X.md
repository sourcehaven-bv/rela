---
id: RR-T36H1X
type: review-response
title: Changes channel cannot fan out
finding: 'Architecture design review: one channel gives each value to one receiver.'
severity: significant
resolution: 'Plan: Service.Subscribe returns a coalescing cap-1 wake channel per subscriber.'
status: addressed
---
