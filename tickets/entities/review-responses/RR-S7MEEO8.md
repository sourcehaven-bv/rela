---
id: RR-S7MEEO8
type: review-response
title: Erased org could be auto-provisioned again
finding: After tenant erase, a valid token for the erased org could auto-provision it again through the lazy or webhook path.
severity: critical
resolution: Added the rela_control.erased tombstone table. Provision refuses an erased org with ErrTenantErased, the request gets 403, and only an operator tenant create clears the tombstone. Commit f2f2dac83.
status: addressed
---
