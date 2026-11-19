---
id: RR-027RUC2
type: review-response
title: Misleading error for hidden plus blank permission
finding: 'hidden: true with permission: blank reported set hidden or permission not both.'
severity: nit
resolution: The both-set check trims the permission, so this case now reports the padded permission. Test case added.
status: addressed
---
