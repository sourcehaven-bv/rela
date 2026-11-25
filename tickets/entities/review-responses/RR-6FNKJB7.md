---
id: RR-6FNKJB7
type: review-response
title: '[security] A guard can disclose one bit of a hidden field'
finding: '[security] A change or predicate guard that reads a field the author cannot see tells the author through the move whether it holds.'
severity: minor
resolution: 'No code change needed: flowcfg already refuses at load any branch or start guard that reads a property a visible: grant restricts (loader.guard). The flows guide now states this rule.'
status: addressed
---
