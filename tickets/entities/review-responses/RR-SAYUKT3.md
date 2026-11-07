---
id: RR-SAYUKT3
type: review-response
title: Required-claims test checks only the status
finding: A swap of user_id and org_id in missingClaim would pass.
severity: nit
resolution: Added wantMissing column asserting the named claim and a case with both missing.
status: addressed
---
