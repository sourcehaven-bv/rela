---
id: TKT-U2R7GU
type: ticket
title: 'Declarative mails: content templates, scheduler + automation triggers, graph-resolved recipients with per-recipient ACL scoping'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Second of three: the declarative layer, built on the SMTP foundation
(TKT-332QZY) and ahead of the extensibility work (TKT-DS1CR6). Covers the
cases an operator actually asks for — a daily digest of overdue tasks, upcoming
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
      where: ["active = true"]
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
    run_as: system:digest
```

**Trigger 2 — automation** (has entity context). `Action` is a small closed
union (`Set` / `CreateRelation` / `CreateEntity`) in
`internal/automation/types.go`; this adds one arm:

```yaml
on: {property: status, to: blocked}
do:
  - send_mail:
      template: blocked_notice
      entity: "{{entity.id}}"
```

**`{{entity}}` is a load-time error on a scheduled trigger.** A template
referencing entity context is automation-only; naming it from `schedules.yaml`
fails the load rather than rendering an empty mail. This follows the house rule
that a `condition:` which fails to compile is a load error — dropping a
constraint silently is the unsafe direction. Detected by scanning the template's
interpolations at config load.

**`style:`** per section — `table` (columns), `list` (titles + deep links), or
`detail` (renders the entity's markdown body, for the meeting-agenda case).

## Recipients and ACL

Recipients resolve from the graph: an `entity_type` + `property` holding the
address, optionally filtered. `group_by:` yields a per-recipient digest (each
person gets their own open tasks) rather than one broadcast.

**Mail renders entity content to an inbox, outside every read gate — it is an
exfiltration surface.** Two rules:

1. A scheduled mail renders through its `run_as` principal's visibility wrapper.
`run_as` is identity, not capability (DEC-O59WM4) — naming a principal grants
nothing; `acl.yaml` decides what it reads.
2. A per-recipient digest renders as **that recipient's** principal, so nobody is
mailed content they could not see in the app. Depends on principal resolution
(FEAT-OF2ZOL, `principal_property`); if that is not ready, per-recipient scoping
falls back to a single `run_as` and `group_by:` is gated off rather than shipped
unscoped.

Reads go through `internal/visibility` decorators at the wiring site — never
per-consumer redaction calls.

## Testing note

The `transport: memory` sender from TKT-332QZY is what makes these criteria
testable without an SMTP fake in every case: a test triggers a digest and asserts
on the recorded messages — recipients, subject, and rendered parts. The ACL
criteria below (6) depend on that, since they assert on what a *specific
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
3. A template using `{{entity}}` referenced from `schedules.yaml` **fails config load**
with a message naming the template and the offending interpolation.
4. The same template referenced from an automation renders with entity context.
5. `group_by:` produces one mail per recipient containing only that recipient's rows.
6. An entity hidden from the recipient by `acl.yaml` does **not** appear in their mail
(row-level), and a `visible:`-redacted property is absent from the rendered
table.
7. A section matching zero entities renders the empty message, not a broken table.
8. Automation-triggered send enqueues without blocking the write, and the write still
commits if the mail server is unreachable.
9. `rela validate` reports unknown template names, unknown entity types, and
unparseable `where:` clauses.
10. Config validation rejects unknown keys with a typo suggestion (the
`checkUnknownKeys` pattern).

## Risks

- **ACL leak via mail** — the central risk. Mitigated by rendering through visibility
wrappers and by an explicit test per criterion 6. If per-recipient principal
resolution is unavailable, ship single-principal only rather than unscoped.
- **Digest fan-out cost** — N recipients means N scoped queries; needs a bound and a
documented cap rather than silent truncation.
- **Template/trigger coupling drift** — mitigated by making the `{{entity}}` mismatch a
load error rather than a runtime surprise.
