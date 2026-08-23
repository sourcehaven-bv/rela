---
id: TKT-U2R7GU
type: ticket
title: 'Declarative mails: content templates, scheduler + automation triggers, graph-resolved recipients with per-recipient ACL scoping'
kind: enhancement
priority: medium
effort: l
status: planning
---

## Description

The declarative layer, built on the SMTP foundation (TKT-332QZY). Automation
triggers split out to TKT-LU4AAY; per-recipient scoping to TKT-XWZIOB. Covers the cases
an operator actually asks for — a daily digest of overdue tasks, upcoming
events, a meeting reminder carrying its agenda, and "this changed, go look" from
an automation — declaratively.

## Design: content and trigger are separate

The scheduler is **context-free**; an automation **always carries an entity**.
Folding both into one `mails:` key would imply any mail can be triggered any
way, which is false. So the content declaration is trigger-agnostic and the
triggers reference it — the same split `feeds:` already makes (a feed declares
content; the URL is delivery).

```yaml
# content only — no trigger, no delivery mechanism
mail_templates:
  overdue_digest:
    subject: "Tasks due {{today}}"
    intro: |
      You have **{{count}}** items needing attention.
    to:
      entity_type: person
      property: email
      where: ["active = true"]   # broadcast: one render, N addresses
    sections:
      - title: "Overdue"
        entity_type: task
        where: ["status != done", "due < today"]
        columns: [title, due, owner]
        link: true
      - title: "Due this week"
        entity_type: task
        where: ["due <= today+7d"]
        style: list
```

`where:` is `[]string` parsed by `filter.ParseAll`, and `columns:` reuses
`ListColumn` — deliberately the vocabulary from `feeds:`/`lists:` rather than a
third dialect.

**Trigger 1 — scheduler** (`schedules.yaml`, context-free):

```yaml
tasks:
  - name: daily-digest
    template: overdue_digest
    every: day
    run_as: system:digest      # ONE identity: the mail renders under it
```

Per-recipient rendering adds `for_each:` here (TKT-XWZIOB); without it the
digest is a broadcast under `run_as`.

**Trigger 2 — automation** is TKT-LU4AAY, along with the `{{entity}}`
load-time validation that only has something to validate once templates exist.

**`style:`** per section — `table` (columns), `list` (titles + deep links), or
`detail` (renders the entity's markdown body, for the meeting-agenda case).

## Recipients and ACL

Recipients resolve from the graph: an `entity_type` + `property` holding the
address, optionally filtered.

**Per-recipient scoping is NOT built here.** It is scheduler fan-out
(TKT-XWZIOB): "run this task once per selected user, as that user" is a
scheduler capability that mail is merely the first consumer of. Building a
mail-shaped version would leave the next consumer — a per-user report, a per-user
cleanup pass — either duplicating it or bending mail's config to reach it.

So this ticket ships two recipient modes, and the second arrives with fan-out:

1. **Broadcast (this ticket).** One mail, rendered once under the task's
   `run_as` principal, sent to every resolved address. `run_as` is identity, not
   capability (DEC-O59WM4) — `acl.yaml` decides what it reads. Correct when every
   recipient is entitled to the same view: an ops digest, a team summary.
2. **Per-recipient (TKT-XWZIOB).** `for_each` fans the task out, and each run
   renders under that user's own principal. A recipient cannot be mailed content
   they could not see in the app.

The distinction has to be **visible in the config**, not implicit, because the
two have different confidentiality properties and a reader must be able to tell
which one they wrote.

Reads go through `internal/visibility` decorators at the wiring site — never
per-consumer redaction calls.

**Note on field-level redaction.** Scheduled jobs currently get row gating only;
`appbuild.ScheduledLuaWriteDeps` wires a nil redactor (RR-7408F5,
`appbuild.go:415-426`). Broadcast mail inherits that limitation and must say so
in the operator guide: a digest may include a property a human with the same
role would see redacted. TKT-XWZIOB closes it, because "sees what that user
sees" cannot be half-enforced.

## Testing note

The `transport: memory` sender from TKT-332QZY is what makes these criteria
testable without an SMTP fake in every case: a test triggers a digest and
asserts on the recorded messages — recipients, subject, and rendered parts. The
ACL criteria below (6) depend on that, since they assert on what a *specific
recipient* did and did not receive.

## Scope: IS NOT

- No WYSIWYG/template editor in the SPA.
- No per-recipient send-time preferences or unsubscribe management.
- No new filter syntax — `filter.ParseAll` as-is.
- No mail preview endpoint (candidate follow-up; would need the same ACL gate).

## Acceptance criteria

1. A `mail_templates:` entry with two sections renders one mail with a table section
and a list section, deep links resolving to the configured base URL.
2. `style: detail` renders the entity's markdown body (agenda case).
3. A template naming an unknown entity type or an unparseable `where:` fails
config load.
4. A scheduled task naming an unknown template fails config load.
5. Recipients resolve from the graph: a template naming `entity_type` + `property`
mails every matching address.
6. Broadcast mail is rendered under the task's `run_as` principal — an entity
that principal may not read does **not** appear (row-level). Per-recipient
scoping and field-level redaction are TKT-XWZIOB's criteria, not this
ticket's.
7. A section matching zero entities renders the empty message, not a broken table.
8. Automation-triggered send enqueues without blocking the write, and the write still
commits if the mail server is unreachable.
9. `rela validate` reports unknown template names, unknown entity types, and
unparseable `where:` clauses.
10. Config validation rejects unknown keys with a typo suggestion (the
`checkUnknownKeys` pattern).

## Risks

- **ACL leak via mail** — the central risk. Broadcast mail renders under one
principal, so the exposure is bounded by what `run_as` may read, and the
operator guide must state that plainly rather than implying per-recipient
scoping. Shipping broadcast-only is deliberate: a half-enforced per-recipient
mode would read like a guarantee it does not provide.
- **Recipient-count blowup** — a broadcast to N addresses is one render and N
sends through a sequential outbox; needs a bound with a loud log rather than
silent truncation.
- **Broadcast mistaken for per-recipient** — the config must make the two modes
visibly different, since they have different confidentiality properties;
TKT-XWZIOB adds the second one.
