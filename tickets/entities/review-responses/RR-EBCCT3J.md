---
id: RR-EBCCT3J
type: review-response
title: Erasure precheck omits recently closed cases under evaluation
finding: Step 1 of the erasure procedure only checked for an open incident or nonconformity. PROCEDURE-u020 also requires that no recently closed incident, NCR or dispute whose evaluation period is still running refers to the tenant.
severity: minor
resolution: Step 1 in GUIDE-postgres-backend, the TKT-TNERAS decision, the erase command's help text and confirmation prompt, and the PR body now require checking that no open or recently closed incident, NCR or dispute refers to the tenant while its evaluation period is still running (PROCEDURE-u020).
status: addressed
---
