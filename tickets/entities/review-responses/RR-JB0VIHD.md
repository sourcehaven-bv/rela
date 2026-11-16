---
id: RR-JB0VIHD
type: review-response
title: cell_colors keys missing from the Configure allowlist
finding: 'TestAllowlist_ClassifiesEveryKey failed: kanbans.*.cell_colors.*.*.color and .label were neither editable nor locked.'
severity: critical
resolution: Added kanbans.*.cell_colors.** to the editable list in internal/configedit/allowlist.go.
status: addressed
---
