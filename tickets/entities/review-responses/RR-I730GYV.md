---
id: RR-I730GYV
type: review-response
title: No Go test for a gated named dashboard
finding: No handler test covered a permission-gated validation toggle on a named dashboard.
severity: minor
resolution: TestPageChrome_NamedDashboardGatedPerPrincipal checks alice (holder) and bob (non-holder) on ?name=portal.
status: addressed
---
