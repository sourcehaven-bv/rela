---
id: RR-CPP788
type: review-response
title: acl and queryplan not tested with named dashboards
finding: UsedPermissions and StaticIndexSpecs now iterate AllDashboards but no test used a named dashboard.
severity: minor
resolution: Added TestDataEntryPermissions_NamedDashboardCards and TestStaticIndexSpecsIncludeNamedDashboardCards.
status: addressed
---
