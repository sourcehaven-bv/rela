---
id: RR-IZBQWW
type: review-response
title: Settings edited the last-opened project
finding: ProjectSettings used d.svc/d.app, the most recently loaded project, not the project of the window it was opened from.
severity: significant
resolution: The menu passes the focused window's project id; every method takes it and resolves it through the registry. Mail settings on a non-active project ask for a reopen. Tested.
status: addressed
---
