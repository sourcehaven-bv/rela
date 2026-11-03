---
id: RR-YF00X7
type: review-response
title: Config-authored filters and sorts can fail whole views
finding: navItemsListQuery and navStatusListQuery forward a list's configured sort and filters, so a configured sort on a field hidden from some principal would fail for that principal. Keep the 400 for client keys and degrade one sidebar entry, not the whole sidebar.
severity: significant
resolution: navStatusRows gates the configured query; its error fails that one sidebar entry as other per-entry errors do. nav_items goes through listPage and degrades per entry too. Client-sent keys still get the 400.
status: addressed
---
