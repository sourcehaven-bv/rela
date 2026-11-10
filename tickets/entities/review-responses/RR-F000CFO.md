---
id: RR-F000CFO
type: review-response
title: 'Design: edge cases'
finding: Resolve of empty org; --tenant without tenant mode; erasing is one-way and must be said at the prompt; usage and error texts still name tenants.yaml.
severity: minor
resolution: CheckOrgID refuses an empty org; --tenant without tenant mode is ErrNotTenantMode; the erase prompt says the stop is immediate and cannot be undone; TestNothingReadsTheTenantMap pins that no code names tenants.yaml outside the legacy refusal.
status: addressed
---
