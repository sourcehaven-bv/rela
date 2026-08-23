---
id: TKT-LU4AAY
type: ticket
title: 'Automation-triggered mail: send_mail action with entity context and load-time {{entity}} validation'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Split out of TKT-U2R7GU so the daily-digest path ships first. That ticket keeps
`mail_templates:`, section rendering, the scheduler trigger, recipient
resolution and per-recipient ACL scoping; this one adds the **reactive**
trigger.

Sequenced second because the scheduler path answers the question operators
actually ask first ("how do I set up a daily reminder"), and because the
`{{entity}}` validation below only has something to validate once templates
exist.

## Scope: IS

**A `send_mail` arm on the automation `Action` union.** `Action` in
`internal/automation/types.go` is a small closed union (`Set`, `CreateRelation`,
`CreateEntity`); this adds one more:

```yaml
on: {property: status, to: blocked}
do:
  - send_mail:
      template: blocked_notice
      entity: "{{entity.id}}"
```

**Entity context in templates.** A template referenced from an automation can
interpolate `{{entity.*}}`, and a `style: detail` section can render the
triggering entity's markdown body.

**`{{entity}}` from a scheduled trigger is a LOAD error.** The scheduler is
context-free, so a template using entity context cannot be scheduled. Naming it
in `schedules.yaml` fails the load with a message identifying the template and
the offending interpolation, rather than rendering an empty mail at 6am. This
follows the house rule that a constraint which fails to compile is a load error
— dropping it silently is the unsafe direction.

Detected by scanning a template's interpolations at config load, which is only
possible once TKT-U2R7GU defines what a template is.

## The reason this is not a trivial arm

Automations run **inside the write path**, so this is where the outbox earns its
keep: `send_mail` must enqueue and return, never dial. A write must still commit
when the mail server is unreachable.

It is also the first automation action with a side effect **outside the store**.
Every existing arm mutates the graph and is covered by the transaction; a sent
mail cannot be rolled back. Worth deciding explicitly whether the enqueue
happens before or after the write commits — enqueueing a notification for a
write that then fails is the failure mode to avoid.

## Scope: IS NOT

- No `mail_templates:` config or rendering — TKT-U2R7GU.
- No scheduler trigger — TKT-U2R7GU.
- No recipient resolution or ACL scoping mechanism — TKT-U2R7GU; this reuses it.
- No retry semantics beyond the existing best-effort outbox.

## Acceptance criteria

1. A status transition matching `on:` sends a mail whose content reflects the
triggering entity.
2. `style: detail` renders the triggering entity's body (the meeting-agenda case).
3. A template using `{{entity}}` referenced from `schedules.yaml` **fails config
load**, naming the template and the interpolation.
4. The same template from an automation renders correctly.
5. The action **enqueues without dialing**: a write completes at normal speed
with an unreachable mail server.
6. A write that fails does not leave a queued notification for it.
7. Mail content is ACL-scoped to the automation's acting principal — an
automation cannot mail content the actor could not read.
8. `rela validate` reports a `send_mail` naming an unknown template.

## Risks

- **Write-path latency** — mitigated by enqueue-only, asserted by criterion 5.
- **Notification for a rolled-back write** — the ordering decision above;
criterion 6 pins whichever way it goes.
- **ACL** — an automation runs on the actor's ctx (DEC-O59WM4), so the scoping
from TKT-U2R7GU applies unchanged. Criterion 7 makes that explicit rather than
assumed.
