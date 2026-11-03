---
id: RR-ZIEB9F1
type: review-response
title: Self-transition puts current in next
finding: sessionView builds next from every transition target, so a self-transition would render 'Working -> Working'.
severity: minor
resolution: nextStates excludes the current state; story and test.
status: addressed
---
