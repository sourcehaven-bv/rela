---
id: RR-MC0I9K
type: review-response
title: Docs claim commands show on dashboards
finding: The guide said scoped commands show on a dashboard but the SPA dashboard page never fetches commands.
severity: significant
resolution: Docs now say the scope applies to /api/v1/_commands?page_type=dashboard&qualifier=<name> and that the dashboard page does not list commands.
status: addressed
---
