---
id: RR-VF95AO
type: review-response
title: create/update --title asymmetry lacks explanatory comment
finding: '`rela update` still has `-t/--title` (writes literal `title` property) while `rela create` no longer does. The asymmetry is defensible (update always wrote literal title, never GetPrimaryProperty) but undocumented. A one-line godoc on UpdateCmd.Title explaining why create has no -t (display property may be a derived multi-property template, TKT-2SVA3L) would save future readers a git-blame session.'
severity: minor
resolution: Added a godoc comment on UpdateCmd.Title (internal/cli/update.go) explaining it writes the literal 'title' property, that update never targeted the display property (so it stays), and that create deliberately has no -t because the display property may be a derived multi-property template (TKT-2SVA3L) — use -P <prop>= there.
status: addressed
---
