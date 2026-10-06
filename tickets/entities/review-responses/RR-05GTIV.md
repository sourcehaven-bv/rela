---
id: RR-05GTIV
type: review-response
title: Exits from the form drop the world
finding: The not-editable Back to entity link and the cold-open cancel fallback link without ?world=.
severity: minor
resolution: The Back to entity link carries formWorld. The cancel fallback goes to a list or the dashboard and normally uses router.back(), which keeps the world; left as is.
status: addressed
---
