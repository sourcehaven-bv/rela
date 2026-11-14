---
id: RR-4DEP7S7
type: review-response
title: Warning order must survive Fire
finding: If Fire evaluates all guards and rendering runs afterwards, condition warnings move ahead of earlier automations' action warnings.
severity: significant
resolution: 'Plan: rendering walks the Fired list in declaration order and emits a failed guard''s warning at that trigger''s position. The synthetic golden pins the Warnings order.'
status: addressed
---
