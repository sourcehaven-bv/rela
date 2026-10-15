---
id: RR-U7TF4J
type: review-response
title: No configedit tests for dashboards.* paths
finding: The allowlist gained dashboards.* patterns but no test proved editable and protective permission behaviour.
severity: significant
resolution: TestCheckEdit_NamedDashboards covers retitle; card edit and add (allowed) and permission change/remove/add and gated card removal (rejected).
status: addressed
---
