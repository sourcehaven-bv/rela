---
id: RR-G2AZ9Q
type: review-response
title: Locked code could be pointed at other records
finding: The trigger keys of an automation or validation that runs Lua or bypasses the ACL stayed editable, so reviewed code could be retargeted.
severity: minor
resolution: 'Pins generalized: automation on and validation entity_type/when/when_condition are pinned while the item holds Lua, capabilities or allow_acl_bypass. Test cases added.'
status: addressed
---
