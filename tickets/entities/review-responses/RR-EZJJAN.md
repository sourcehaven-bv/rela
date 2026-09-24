---
id: RR-EZJJAN
type: review-response
title: related() refused on only three surfaces
finding: related() still compiles in ACL when, automations, validations, statemachine and CLI filter and fails on evaluation; docs implied it is enforced everywhere.
severity: minor
resolution: Docs now state rela validate refuses it in data-entry conditions and that other surfaces fail on evaluation. Behaviour on those surfaces is unchanged from before this ticket.
status: addressed
---
