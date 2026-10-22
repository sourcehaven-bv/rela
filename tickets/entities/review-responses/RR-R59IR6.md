---
id: RR-R59IR6
type: review-response
title: The client-ceiling row in AC 3 tests nothing
finding: 'The ceiling check is CeilingAdmitsUpdate(anchor.Type) for every role source on a write (internal/realtime/authz.go:270-272), and mayPost uses it too. A ceiling that denies update therefore also empties post, and transitions is [] because post[mt] is false, whether fireable applies the ceiling or not. Plan change: replace the row with a case where the post role survives and the transition role does not. For example, a fixture transition with roles: [dpo] whose type members may post, plus a ceiling with deny_permissions: [privacy.officer] (the permission axis, internal/realtimecfg/compiled.go:272-275). Expected: the type is in post but not in transitions.'
severity: minor
resolution: 'Plan updated: AC 3 ceiling row replaced: a roles: [dpo] transition on a member-postable type with deny_permissions: [privacy.officer]; expected in post but not in transitions.'
status: addressed
---
