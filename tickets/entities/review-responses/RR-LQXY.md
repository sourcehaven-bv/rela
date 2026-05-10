---
id: RR-LQXY
type: review-response
title: loadUserLogo must document writeMu requirement
finding: theme_logo.go::loadUserLogo currently safe because it's only called during App boot (before serving) — but a future watcher-driven reload could race against PUT/DELETE. Add a comment marking the function as boot-only or writeMu-only.
severity: nit
resolution: 'Added concurrency comment to loadUserLogo godoc: NOT safe to call in parallel with saveUserLogo/deleteUserLogo; boot path is fine because the App isn''t yet serving, future reload paths must hold writeMu.'
status: addressed
---
