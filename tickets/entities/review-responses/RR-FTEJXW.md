---
id: RR-FTEJXW
type: review-response
title: 'by: trigger is accepted where no trigger can fire it'
finding: 'internal/realtimecfg/session.go:223 area: a `by: trigger` transition on a `cardinality: many` channel type loads. Triggers may only post into cardinality-one types (validate.go:772), so that transition can never fire. Neither the load check nor docs/realtime.md "Who can move a session" says so. Fix: refuse it with a reason ("a trigger posts only into cardinality one channel types, so by: trigger never fires here"), or add a load warning when no trigger posts that message type to this channel type. Add a TestParse_SessionInvalid row.'
severity: minor
resolution: 'A by: trigger transition in a cardinality: many type is refused at load; tested in TestParse_SessionInvalid; the guide mentions it.'
status: addressed
---
