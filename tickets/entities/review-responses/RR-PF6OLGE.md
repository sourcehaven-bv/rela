---
id: RR-PF6OLGE
type: review-response
title: Assign action bypasses action permission
finding: POST /_assign with a list ignores the action's permission/available_on/when.
severity: significant
resolution: 'Load now refuses permission, available_on and when on an assign action: the action is a key for the list''s assign and the write is gated by update permission.'
status: addressed
---
