---
id: RR-R58UNV
type: review-response
title: PermitsProjectWrite godoc described only git sync
finding: The name and godoc spoke of acting on the whole project, but the settings save writes shared UI state. (cranky-code-reviewer)
severity: nit
resolution: The godoc now defines the decision as a write to state shared by every principal and names both git sync and the shared settings.
status: addressed
---
