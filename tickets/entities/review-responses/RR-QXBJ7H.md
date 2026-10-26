---
id: RR-QXBJ7H
type: review-response
title: '{{message.author}} rule for trigger-reachable transitions is self-contradictory'
finding: 'Approach 3 (plan:292-296) allows {{message.author}} ''on a trigger-reachable transition with roles''. A transition with roles is never trigger-reachable: the check runs only for len(raw.Roles)==0 && By==any (internal/realtimecfg/session.go:416-422), and poster.admits never passes a trigger when roles are named (internal/realtime/session.go:194-210). On a real trigger-reachable (no-roles) transition the job move writes the literal system:realtime into the property with run_as authority, which is the same thing the plan refuses on by: trigger. Plan change: refuse {{message.author}} on every output that can run as a job move (by: trigger and trigger-reachable), with the same reason, or fix the sentence to say no-roles and document the system:realtime value. Simplest is to refuse it.'
severity: minor
resolution: '{{message.author}} is refused on every output that can run as a job move (by: trigger, trigger-reachable, timer), with one reason. The contradictory sentence is removed.'
status: addressed
---
