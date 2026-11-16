---
id: RR-PTR0U5E
type: review-response
title: YAML anchors and aliases would dangle after the move
finding: flowcfg refuses anchors and aliases; an alias inside automations would point at an anchor left behind.
severity: minor
resolution: 'Plan: the migration refuses aliases and anchors in moved entries.'
status: addressed
---
