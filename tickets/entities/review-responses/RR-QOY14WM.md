---
id: RR-QOY14WM
type: review-response
title: 'Design: scalar-or-list keys need list patterns'
finding: on.entity on.where and notify.to.via take a scalar or a list; patterns must use .[] or list edits are refused.
severity: minor
resolution: on.entity and on.where use list patterns (triggers.*.on.entity.[]). TestCheckEdit_Flows covers a scalar entity changed to a list.
status: addressed
---
