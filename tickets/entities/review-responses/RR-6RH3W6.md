---
id: RR-6RH3W6
type: review-response
title: gated() passed the cascadeWrite bypass through
finding: '[security] gated() returned the receiver whenever bypassACL was false, so on a cascadeWrite handle it would hand the trigger-authorized bypass to a nested cascade or script. Not reachable today because DeleteEntity dispatches no automations.'
severity: minor
resolution: gated() returns a fresh handle when either bypassACL or cascadeWrite is set. Pinned by TestGated_StripsEveryBypass.
status: addressed
---
