---
id: RR-KIBS1T
type: review-response
title: 404 for missing pile and unreadable item collide
finding: On POST items the SPA cannot tell 'pile gone' (close the panel) from 'item not readable'.
severity: minor
resolution: 'Plan: 404 pile_not_found for the pile, 404 item_not_found (never naming which) for items.'
status: addressed
---
