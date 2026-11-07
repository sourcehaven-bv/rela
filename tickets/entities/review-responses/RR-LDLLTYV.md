---
id: RR-LDLLTYV
type: review-response
title: Elevated IdP webhook action fails at runtime
finding: The webhook never sets ElevatedUpdater, so an elevated script failed at bypass_acl.
severity: minor
resolution: The webhook refuses an action with allow_acl_bypass before the script runs. TestWebhook_RefusesElevatedAction.
status: addressed
---
