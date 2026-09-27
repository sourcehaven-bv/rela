---
id: RR-QZ6N76
type: review-response
title: 'Transition when: traversal over a conditionally visible field is silent'
finding: '[security] A state-machine transition when: that traverses to a field hidden by a conditional visible: grant gave no load-time signal.'
severity: minor
resolution: WithMachines warns via warnConditionallyVisible for each traversing when; TestResolver_WithMachinesWarnsOnHiddenTraversalFilter.
status: addressed
---
