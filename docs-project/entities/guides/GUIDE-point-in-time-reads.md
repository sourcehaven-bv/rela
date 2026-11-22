---
id: GUIDE-point-in-time-reads
type: guide
title: "How To Read the Data as It Stood on a Date"
status: published
order: 33
audience: intermediate
summary: "Read a world as it stood at a past instant with ?as_of=, from the API or the web app's date picker, for audits and reviews"
---

## Introduction

An auditor may ask what a record looked like on a past date. For example:
"What did the statement of applicability look like on 31 December?"

rela answers this from its version history. A request can name an instant,
and every read in that request shows the data as it stood at that instant.
This is called a point-in-time read, or an as-of read.

A point-in-time read is read-only. It shows the past; it never changes it.

## Prerequisites

- The PostgreSQL or SQLite backend. Both keep version history in the
  database. The file and memory backends keep none, so they refuse a
  point-in-time read with a clear error instead of showing today's data.
- The `history:read` permission. A read of the past is a history read, so a
  role must grant it, for example `permissions: [history:read]` in
  `acl.yaml`. Without it, every point-in-time read answers
  `403 as_of_forbidden`.
- Version history that covers the instant. Rows written before the backend
  started keeping history have no versions, so the read can only use their
  current state when it is older than the instant.

## Naming the Instant

Add `?as_of=` to a read. The value is an RFC 3339 timestamp with an explicit
zone:

```text
GET /api/v1/controls?as_of=2025-12-31T23:59:59Z
GET /api/v1/controls?as_of=2025-12-31T23:59:59.999+01:00
```

A bare date such as `2025-12-31` is refused, because it does not say which
zone the day is in. The web app turns a picked date into the end of that day
in the browser's zone before it sends it.

`as_of` combines with `world`. The world decides which face of each entity
the read serves. The instant decides which state of the data it reads.

```text
GET /api/v1/controls?world=published&as_of=2025-12-31T23:59:59Z
```

## What the Read Shows

At the instant, each entity and relation is shown in the state it had then:

| Situation | What the read shows |
| --- | --- |
| Created after the instant | Nothing. A detail read returns 404. |
| Changed after the instant | The values it had at the instant. |
| Deleted after the instant | The entity, as it was at the instant. |
| Deleted before the instant | Nothing. |
| A face published after the instant | Nothing on that face. A world that prefers it falls back to the next face it allows. |

Access control is today's. The instant changes which rows exist, not who is
asking or what the policy says. A row you cannot read today stays hidden in
the past too. Access you lost since the instant does not let you read the
past either. A record deleted since the instant has no relations today, so
you see it only if you may read every record of its type.

A filter that follows a relation, such as `related(...)` in a list
condition, follows the relations as they stood at the instant. Whether you
may read the record it reaches is still decided by today's access rules.

Field redaction is stricter than on a live read. It follows the rules of the
history view: a field is shown only when a role you hold for the whole type
grants it. A field you see live only through a relation, such as being the
owner of the record, is hidden in a past read. A field condition on the
record's own values, such as its classification, is judged on the values at
the instant.

## Which Reads Answer at an Instant

These API reads accept `as_of`:

- entity lists, which also feed kanban boards and calendars;
- entity detail (`/{plural}/{id}`) and its relations
  (`/{plural}/{id}/relations`);
- entity views (`/_views/...`);
- dashboard cards (`/_dashboard/card`);
- search (`/_search`);
- gantt charts (`/_gantts/{id}`);
- documents (`/_documents/{name}/{id}` and standalone documents). A document
  rendered at an instant is never served from, or stored in, the render
  cache.

Any other endpoint refuses `as_of` with `422 as_of_unsupported`, so a read
never mixes today's data into a past state without saying so. The schema,
configuration and sidebar describe the app, not the data. The web app reads
them as they are now.

## Errors

| Status | Code | Meaning |
| --- | --- | --- |
| 400 | `as_of_invalid` | The value is not an RFC 3339 timestamp with a zone. |
| 400 | `as_of_duplicated` | `as_of` was given more than once. |
| 403 | `as_of_forbidden` | You do not hold the `history:read` permission. |
| 422 | `as_of_read_only` | The request writes. Every write under `as_of` is refused. |
| 422 | `as_of_unsupported` | This endpoint cannot read a past state. |
| 501 | `as_of_backend_unsupported` | The backend keeps no version history. |

Under `as_of`, every verb in `_actions` is `false`. A response offers no
detail actions, transitions, copies or conversions, so a client hides its
write controls.

A document script rendered at an instant cannot call bindings that act
outside the store or read the version history directly: `rela.write_file`,
`rela.attach_file`, `rela.history`, `rela.piles.add` and similar, and the
`ai`, `http` and `mail` modules. Such a call raises an error.

## Using the Web App

When the backend supports it, the sidebar shows a date picker below the
world switcher. Pick a date to see the data at the end of that day. The page
reloads and every view, list, board, calendar, gantt chart, dashboard card,
search and document shows that date.

A banner names the date while you look at the past. Choose **Back to now** to
return to the present. The date lives in the page address (`?as_of=...`), so
you can bookmark it or send it to a colleague.

While a date is set, the web app makes no changes to the data and runs no
commands. Your own settings, such as the theme, can still be saved.

## Example: The Statement of Applicability on 31 December

Suppose the ISMS keeps its controls as `control` entities and the statement
of applicability (SoA) is a list or document over them. Each control carries
`applicable`, `justification` and `implementation_status` properties. An
auditor asks for the SoA as it stood at the end of the year. The auditor's
role grants `history:read`.

**In the web app.** Open the SoA list or document. In the sidebar date picker,
choose 31 December 2025. The page now shows each control with the values it
had at 23:59:59.999 in your zone on that day. Controls added in January are
absent. A control removed in January is still listed. Export or print the
page for the audit file.

**Over the API.** Ask for the published world at the end of the year in the
organisation's zone:

```bash
curl -s "$RELA/api/v1/controls?world=published&as_of=2025-12-31T23:59:59.999%2B01:00" \
  -H "Authorization: Bearer $TOKEN"
```

Render the SoA document at the same instant:

```bash
curl -s "$RELA/api/v1/_documents/soa?as_of=2025-12-31T23:59:59.999%2B01:00" \
  -H "Authorization: Bearer $TOKEN"
```

Encode `+` as `%2B` in the query string, or use a `Z` instant.

**Comparing with today.** Run the same request without `as_of` to see the
current SoA. The difference shows what changed since the audit date.

## Limits

- Comments, presence, the inbox and channels are about the present. They are
  not shown at an instant. Neither are backlinks (mentioned by), leases and
  attachments, which keep no history.
- History before the backend kept versions is not available.
- The version sweep records a change a short while after it is made. An
  entity created and then changed or deleted before the sweep ran has no
  version for its first state, so a read at an instant inside that window
  does not show it.
- A soft-deleted entity is hidden only while it is marked. After it is
  restored or purged, a read at an instant inside the marked period shows it.
- A point-in-time read uses today's schema and configuration. A property
  removed from the schema since the instant is not shown.

## Conclusion

Add `?as_of=` to a read, or pick a date in the sidebar, to see the data as it
stood then. The read is read-only, gated by today's access rules, and refused
where rela cannot answer it at the instant.
