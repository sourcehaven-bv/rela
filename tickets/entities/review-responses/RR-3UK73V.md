---
id: RR-3UK73V
type: review-response
title: setTestACL ordering is prose only
finding: Call before NewRouter is not enforced.
severity: nit
reason: Enforcing it needs a router-built flag on the production App, and PR 6 of TKT-WFS778 removes App.acl and App.fieldResolver together with these helpers. The rule holds today (no call follows NewRouter in any test function).
status: wont-fix
---
