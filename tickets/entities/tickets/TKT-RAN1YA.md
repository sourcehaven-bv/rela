---
id: TKT-RAN1YA
type: ticket
title: 'Sessions 4/5b: timer and trigger outputs (run_as)'
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

Timer- and trigger-fired session transitions get graph `outputs:`, written as
the session type's `run_as` principal, which `acl.yaml` must grant (checked at
load). Follow-up to TKT-98TR9A (5a: message-driven outputs), split off in its
design review.

### Scope (from the 5a plan)
- `session.run_as` (a `system:` or `integration:` name). realtimecfg
checks the prefix itself, because `principal.ValidateRunAs` accepts plain names.
It refuses `system:realtime` and other rela identities, and warns when `run_as`
is unused.
- Outputs on `after.<name>` timer transitions and on trigger-reachable
transitions, written as `run_as`. Timer outputs refuse `{{message.*}}` and
`{{channel.title}}`.
- The load-time grant check. It needs a new `acl.Policy.PrincipalGrants`
for create, update and delete, plus a separate read helper over `RoleDef.Read`
and worlds, because `acl.Op` has no read verb. It must count the built-in
`everyone` role. It must document that a run_as whose only grant is a local role
is refused at load. No `ceilingguard_test.go` edit is needed, because
`policy.go` is exempt as a file.
- What an acl.yaml reload that drops a run_as grant does. Either accept
that the whole realtime config degrades and document it, or treat it as a
per-type problem whose timer outputs fail `forbidden`.
- Timer marks created unclaimed by `SetSession` (the `Mark` callback on
the timer step), the runner woken by `TimerReport.Marks`, and the timer writer
rule. The writer comes from the current config's `run_as`, or the mark fails
`run_as_changed`; the stored `Writer` is for display only.
- testdata: `system:breach-bot` with a `breach-bot` role in
`testdata/acl.yaml`, and the `notify-deadline` output `escalated: true` in the
example.

Design: RES-O8I8S7 (decisions section). Part of FEAT-GWF8YH phase 4.
