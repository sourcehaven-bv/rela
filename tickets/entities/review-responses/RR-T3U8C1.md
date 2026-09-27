---
id: RR-T3U8C1
type: review-response
title: Legacy import silently ignores read errors
finding: importLegacyState returns on any Get error without logging, so a permission or IO error hides a pending import.
severity: minor
resolution: importLegacyState logs an ERROR for any read error other than not-exist.
status: addressed
---
