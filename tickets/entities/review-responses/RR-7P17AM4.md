---
id: RR-7P17AM4
type: review-response
title: Main window lock is a package global
finding: mainWindowMu was a package-level mutex beside per-Desktop state.
severity: nit
resolution: The mutex moved onto the mainWindows field of Desktop.
status: addressed
---
