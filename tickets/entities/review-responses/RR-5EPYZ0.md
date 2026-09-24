---
id: RR-5EPYZ0
type: review-response
title: State-machine read side gated but write side raw
finding: Gating _transitions while the write reads raw offers a transition the write then refuses with 422; the gate moves the one-bit channel rather than closing it.
severity: significant
resolution: 'Plan changed: both sides read raw; the one-bit channel is documented once in acl-security. Commit 9 (gated Performable) dropped.'
status: addressed
---
