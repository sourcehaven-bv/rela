---
id: RR-HUBH5I
type: review-response
title: 'Nits: md name collision, context, simpler pg hash'
finding: A transform named md would be shadowed; binding must use runtime ctx; pg can hash the buffer directly.
severity: nit
resolution: 'Plan: a registered transform named md wins and this is documented; runtime ctx used; pg hash is a generated column so no Go hash there.'
status: addressed
---
