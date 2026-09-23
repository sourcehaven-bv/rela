---
id: RR-AT5DKH
type: review-response
title: Sidebar item key collision and label-less error messages
finding: 'Sidebar.vue:188 keys items on label + href/action; two entities: entries in one group share an empty key. validateNavEntry names entries by label (validate.go:540).'
severity: minor
resolution: 'Key entities: items on type + scope + index. Validation errors for entities: entries name the entity type.'
status: addressed
---
