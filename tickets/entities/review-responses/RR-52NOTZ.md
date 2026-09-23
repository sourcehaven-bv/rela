---
id: RR-52NOTZ
type: review-response
title: Collapsed sidebar shows a column of identical entity icons and hides failures
finding: Collapsed each entity row kept only its icon (up to 100 identical glyphs) while the overflow and failure notes were hidden as labels.
severity: minor
resolution: Entity rows are hidden while the sidebar is collapsed (restored in the mobile layout like other labels). Failed and empty now look the same only because the whole entry is absent in that mode. Documented in the data-entry guide.
status: addressed
---
