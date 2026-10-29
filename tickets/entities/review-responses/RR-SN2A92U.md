---
id: RR-SN2A92U
type: review-response
title: Commands on a $user list receive every user's rows
finding: commands.go applyFilters skipped every $ value, so a command on a my-rows list ran on all readable rows.
severity: significant
resolution: applyFilters takes the user (filterUser(r.Context())) and resolves $user; with no identified user it matches nothing. TestApplyFilters cases added; docs say so.
status: addressed
---
