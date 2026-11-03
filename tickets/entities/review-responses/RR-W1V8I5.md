---
id: RR-W1V8I5
type: review-response
title: waiting_for on broadcast roles (attached, principal_type, permission) defeats the cheap, gated inbox the plan promises
finding: '`call.can` answers `SourceAttached` as true for every reader who passes the ceiling (`realtime/authz.go:243-244,253-255`). `waiting_for: [member]` therefore puts every reader of every anchor on "waiting on me", and the inbox query becomes a full scan of channels in those states with a row gate per row. Permission roles need `HoldsPermissionForEntity` per anchor (`appbuild/realtime_service.go:261-263`), which is a lookup per row. Approach 9''s third bullet ("calls it per row with no extra lookups") contradicts its second bullet. Change: decide now. Either refuse `from: attached` (and possibly `principal_type`) in `waiting_for` with a reason, or record that the inbox lists only named roles and that `waiting_on_me` stays true for the others. Fix the contradictory bullet so it says permission roles need one batched ACL query per page.'
severity: significant
resolution: 'waiting_for takes only bounded roles (from: entity.<prop>, creator); attached, permission and principal_type are refused with a reason (AC 5); inbox section made consistent.'
status: addressed
---
