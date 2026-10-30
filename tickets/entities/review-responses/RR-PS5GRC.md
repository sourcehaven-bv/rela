---
id: RR-PS5GRC
type: review-response
title: Reload tests have no row for adding or removing offer
finding: 'internal/realtimecfg/session_test.go:256-279 lists the session edits that CheckReload accepts (for example "on_final removed"). The plan''s edge-case list says a reload that adds or removes offer is accepted, but no test pins that. Change: add rows "offer added" and "offer removed" to that table under AC 1 or AC 4.'
severity: minor
resolution: Added the CheckReload rows "default_in added" and "default_in removed" to internal/realtimecfg/session_test.go under AC 1.
status: addressed
---
