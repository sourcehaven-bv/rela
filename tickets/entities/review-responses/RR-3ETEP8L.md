---
id: RR-3ETEP8L
type: review-response
title: Rows read are not capped before the limit
finding: Children read all gated related rows before the per-parent cap.
severity: minor
reason: Gate-then-truncate needs the gated set; the guide says rows read grow with related rows.
status: wont-fix
---
