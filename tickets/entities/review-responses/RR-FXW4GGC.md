---
id: RR-FXW4GGC
type: review-response
title: Empty composer wrapper when every type is unoffered
finding: rl-channel__composer keeps its padding when the composer renders nothing.
severity: nit
reason: Only happens in a channel where people post nothing, which is the operator's explicit choice; the padding is cosmetic and the existing caller-decides contract covers it.
status: wont-fix
---
