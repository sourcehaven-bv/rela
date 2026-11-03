---
id: RR-4M9HLP
type: review-response
title: The AC 5 budget row pins nothing
finding: 'The only session fixture in channels_budget_test.go (sessionThreads, internal/dataentry/channels_budget_test.go:281-295) has no roles: on its transitions, so fireable asks no role and the head costs the same by construction. Comparing against "a channel without a session" is also the wrong target: a permission-sourced transition role legitimately calls HoldsPermissionFor. Plan change: give the session fixture transitions whose roles come from a permission and from an entity property, shared with a post role. Pin that the head''s entity-store and channel-store calls do not change with the number of states and transitions, and that each distinct role is asked once (count the calls through the acc fake).'
severity: minor
resolution: 'Plan updated: AC 6: sessionThreads gains permission- and entity-property transition roles shared with a post role; pins store calls equal for 2 vs 10 states and each distinct role asked once via the acc fake.'
status: addressed
---
