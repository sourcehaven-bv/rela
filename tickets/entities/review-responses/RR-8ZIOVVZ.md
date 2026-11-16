---
id: RR-8ZIOVVZ
type: review-response
title: Validation of an unknown list on an entity dashboard is untested
finding: validateEntityDashboard skips the relation check for an unknown list and relies on validateDashboardCardSource to report it.
severity: minor
resolution: Added the unknown list case to TestValidateEntityDashboards.
status: addressed
---
