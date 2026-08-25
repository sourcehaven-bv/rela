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

The declarative layer, built on the SMTP foundation (TKT-332QZY), the shared job
queue (TKT-YOED3R), and scheduler fan-out (TKT-XWZIOB). Automation triggers are
split out to TKT-LU4AAY. Covers the cases
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
# content only — no trigger or recipient selection
mail_templates:
  overdue_digest:
    subject: "Tasks due {{today}}"
    intro: |
      You have **{{count}}** items needing attention.
    address_property: email
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
    for_each:
      entity_type: person
      where: ["active = true"]
```

Mail tasks require `for_each:`. There is deliberately no broadcast fallback:
the selected entity is resolved to a principal and the message is rendered as
that principal. A shared `run_as` would prove only that the sender may read the
content, not that every recipient may read it.

**Trigger 2 — automation** is TKT-LU4AAY, along with the `{{entity}}`
load-time validation that only has something to validate once templates exist.

**`style:`** per section — `table` (columns), `list` (titles + deep links), or
`detail` (renders the entity's markdown body, for the meeting-agenda case).

## Recipients and ACL

`for_each` resolves recipients from the graph. The template names the property
on that recipient entity that holds its delivery address.

**Address validity is asserted from config, not discovered at send time.** Since
the config names the entity type and the property, the selection can be checked
up front: every entity the recipient query yields must have a non-empty and
unambiguous value for that property. An operator who mistypes the property name,
or whose `person` records have blank emails, should learn about it when the
config is validated — not from a partial send at 6am.

Two consequences for where this lives. It is graph-dependent, so it cannot go in
a syntactic config check (`scheduler.Config.validate`, `config.go:195`, never
touches the store); it needs a store-aware surface, alongside the other
`rela validate` graph checks. And it belongs to **mail**, not to the scheduler:
`for_each` (TKT-XWZIOB) resolves an entity to a principal, and a per-user export
or cleanup pass has no address at all. A blank address must not be fatal to the
whole run — the mail for the other recipients still goes out, and the skipped
one is named in the log.

Execution is two-phase. The scheduler posts one expansion job for the task
occurrence. That job queries the bounded `for_each` selection and posts one
delivery job per recipient. Each delivery job independently resolves the
recipient principal, installs its ACL request, resolves the current address,
queries content, renders, and sends. The payload carries stable identifiers
(task, template, occurrence, recipient entity, principal), never rendered bytes
or an email address.

The expansion succeeds once every intended child is accepted; it does not wait
for delivery. A child failure retries only that recipient. Persistent
occurrence-level child claims prevent an expansion retry from recreating a
delivery that already completed; the queue's pending-only `IdempotencyKey` is
not sufficient because its key becomes reusable after completion.

Reads go through `internal/visibility` decorators at the wiring site — never
per-consumer redaction calls.

Field-level redaction is mandatory, not a documented limitation. TKT-NJ91LX
must land before delivery: "sees what that user sees" cannot be half-enforced.

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
4b. A `for_each` selection whose entities lack a usable address is reported by
`rela validate`, naming the property and the offending entities, rather than
surfacing as a partial send. At send time a blank address skips that recipient
with a named log line and does not fail the others.
5. `for_each` posts one delivery job per matching recipient, and each job sends
only to the address on its recipient entity.
6. Every delivery renders under its recipient principal: a denied entity and a
`visible:`-redacted field are absent from that recipient's message.
7. A section matching zero entities renders the empty message, not a broken table.
8. Expansion does not wait for delivery; one delivery failure retries only that
recipient and does not stop later scheduler tasks or replay successful peers.
8b. Retrying an expansion after one child completed does not enqueue that child
again; task + occurrence + recipient is a persistent delivery identity.
9. `rela validate` reports unknown template names, unknown entity types, and
unparseable `where:` clauses.
10. Config validation rejects unknown keys with a typo suggestion (the
`checkUnknownKeys` pattern).

## Risks

- **ACL leak via mail** — the central risk. There is no broadcast mode. Every
delivery installs the resolved recipient's row and field visibility before any
content query, and an end-to-end denial test pins the boundary.
- **Recipient-count blowup** — expansion is bounded and logs/counts what was not
posted rather than silently truncating.
- **Duplicate sends after expansion retry** — pending-job dedup is insufficient;
persistent occurrence-level child claims prevent completed deliveries from being
recreated. SMTP itself remains non-transactional, so a process loss after DATA
but before completion can still duplicate a retry and is documented honestly.
