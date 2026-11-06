---
id: RR-ZK45QD2
type: review-response
title: Automation output bypasses state machine on copy
finding: Properties set by an automation after a copy were not checked against the transition rules.
severity: significant
resolution: applyCopyAutomations runs Transitions.EnforceUpdate on the folded state; TestCopyAutomationTransitionChecked.
status: addressed
---
