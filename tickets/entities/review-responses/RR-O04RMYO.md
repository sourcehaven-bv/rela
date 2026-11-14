---
id: RR-O04RMYO
type: review-response
title: Per-face header read used the gate error class
finding: A failing per-face header read was wrapped as errACLListQuery on the gated branch, though it is a plain load.
severity: minor
resolution: The per-face read now wraps errListLoad on both branches.
status: addressed
---
