---
id: RR-WAZD8W
type: review-response
title: Empty offers array on every entity response
finding: '_task_item_conversions: [] appears on every response, including history versions; return nil when no recipe matches.'
severity: nit
resolution: Offers are now omitted when no recipe starts from the entity type. Goldens regenerated.
status: addressed
---
