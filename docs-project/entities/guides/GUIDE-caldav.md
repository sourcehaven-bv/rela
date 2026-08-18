---
id: GUIDE-caldav
type: guide
title: "CalDAV: syncing to-dos with Apple Reminders and other clients"
status: published
order: 23
audience: intermediate
summary: "Sync to-do collections two-way with Apple Reminders and other CalDAV clients"
---

rela can serve entity collections as CalDAV calendars, so a native to-do app
syncs against the graph two-way: check something off in Reminders and the
markdown updates; edit the markdown and it appears in Reminders.

This guide covers a real deployment. For what the `caldav:` config block means,
see [data-entry configuration](data-entry.md).

## Topology: rela owns no CalDAV-specific auth or transport

```text
CalDAV client --TLS--> Pratique --X-Auth-Assertion--> rela
```

rela's CalDAV endpoint is a JWT-gated route under `/api/`, mounted there
precisely so it inherits the ACL request context, the read gate, the JWT gate
and principal stamping that every other API route has. There is no
CalDAV-specific middleware, no CalDAV credential store, and no TLS code.

That has a consequence worth stating up front, because it is the single most
common setup failure:

> **CalDAV requires a proxied deployment.** `rela-server --project .` on its own
> cannot serve Reminders. macOS will not send credentials over plaintext, and
> rela has no credential subsystem by design.

The two components split as follows.

| Concern | Where | Why |
|---|---|---|
| TLS termination | Pratique | Terminated in-process (cert files or ACME) |
| Authentication | Pratique | Its PAT class was designed for CalDAV clients |
| Authorization / identity | rela | Verifies the ES256 assertion against Pratique's JWKS |

**Pratique consumed the Basic credential upstream.** By the time a request
reaches rela there is no username or password on it at all — only a signed
assertion in `X-Auth-Assertion`. rela never sees the token.

## 1. Configure the CalDAV collection

In `data-entry.yaml`:

```yaml
caldav:
  static:
    tasks:                        # the URL segment: /calendars/tasks/
      meta:
        name: "rela Tasks"        # the list name the client displays
      component: vtodo
      entity_type: task
      where: ["status != cancelled"]
      due: due
      summary: title
      description: notes
      completion:
        status_property: status
        completed_value: done
        pending_value: todo
        completed_at: completed_at
      defaults:
        status: todo
      on_delete:
        set:
          status: cancelled
```

Collections live under `caldav.static:` — "static" because one key is one
collection. The key is the **id**: it becomes the URL segment and the internal
alias key, so it must stay stable (users paste the URL into their client).
`meta.name` is the **label**, and is free to change at any time.

The nesting exists because a second kind is planned — collections generated from
the graph, one per entity of a driver type, where a key would name a *pattern*
that expands rather than a collection that exists. `caldav.dynamic:` is not
implemented and does not parse yet.

### Field mapping

| Client field | iCalendar | Config key |
|---|---|---|
| Title | `SUMMARY` | `summary:` (defaults to the type's display property) |
| Due date | `DUE` | `due:` |
| Start date | `DTSTART` | `start:` |
| Notes | `DESCRIPTION` | `description:` (a property, or `body`) |
| Completion | `STATUS`+`COMPLETED`+`PERCENT-COMPLETE` | `completion:` |
| Priority | `PRIORITY` | `priority:` (integer) or `priority_map:` (enum) |
| Location | `LOCATION` | `location:` |
| Categories / tags | `CATEGORIES` | `categories:` |
| Recurrence | `RRULE` | `rrule:` — **read-only** |

Any of these except recurrence can be made read-only per collection with
[`read_only:`](#read_only--keep-a-third-party-app-from-rewriting-your-content).

Not mapped: `VALARM` (reminders), `ATTACH` (attachments) and `CLASS` (privacy).
A client can display and edit those fields, but the values are dropped rather
than stored.

`rrule:` is read-only in the same sense `feeds:` is: the value is either a
literal the operator set or a property rela owns, so a recurrence edited in a
client has nowhere to land and is discarded on the next sync.

### `priority:` — integer, or bucketed onto an enum

`PRIORITY` is an RFC 5545 integer 0-9 (1-4 high, 5 normal, 6-9 low, 0
undefined). `priority: <integer property>` maps it straight through.

Most projects model priority as an enum instead, so `priority_map:` buckets the
range onto arbitrary values:

```yaml
    priority_map:
      property: urgency
      buckets:
        - {value: high,   from: 1, to: 4, emit: 1}
        - {value: normal, from: 5, to: 5}
        - {value: low,    from: 6, to: 9, emit: 9}
```

Each bucket claims a **range**, because clients pick their own number inside a
band — Thunderbird sends `1` for its "high", Apple Reminders sends `9` for its
"low". Inbound, the first bucket containing the received value wins. Outbound,
`emit:` is the number rendered (defaulting to `from:`).

**The buckets must cover 1-9.** A gap is rejected at startup: a client sending
an uncovered number would write nothing, and the user's change would silently
revert on the next sync. `0` (undefined) is deliberately not required — a
to-do with no priority leaves the property untouched rather than guessing.

### `description:` — a property, or the entity body

`description: notes` maps DESCRIPTION to a property. `description: body` maps it
to the entity's **markdown body** instead:

```yaml
    description: body       # DESCRIPTION <-> the entity's markdown body
```

The body is usually the right target. DESCRIPTION is the one free-text,
multi-line field a to-do has, and the body is where rela keeps multi-line prose
— a `string` property renders as a single-line input everywhere else in the app,
so routing a client's notes into one puts a paragraph in a text box.

`body` is a reserved word here. If your entity type genuinely has a property
called `body`, the config is rejected rather than silently resolved one way.

**Formatting is not preserved.** A client that offers rich-text notes (Mozilla
Thunderbird does; Apple Reminders does not) sends the markup as an
`ALTREP="data:text/html,..."` parameter alongside the plain text. rela reads the
plain value, which RFC 5545 makes authoritative, and does not re-emit the HTML —
so bold, italics and lists entered in a client are flattened on the next sync.
Line breaks and emoji survive.

This is the deliberate choice, not an oversight. Thunderbird is the **only**
task client known to render `ALTREP`; the Microsoft-adjacent ones use a
different mechanism (`X-ALT-DESC`) entirely, and Tasks.org, jtx Board, DAVx⁵,
KOrganizer and Nextcloud Tasks all **strip `ALTREP` when they write a task
back** — so stored HTML would be destroyed by the first edit from an Android
client. **Markdown in the plain value is what survives everywhere**, and several
clients render it. See [CalDAV client compatibility](caldav-clients.md) for the
per-client table and sources.

### `read_only:` — keep a third-party app from rewriting your content

`read_only:` is a **containment** control, not an authorization one. A CalDAV
client is software the operator does not control: it decides what to send, when
to retry, and how much of a to-do to reconstruct from its own local model.
Anything it maps badly, normalizes, or invents lands in your entities. Marking a
field read-only removes that exposure for that field — the value is still
projected outward, but nothing a client sends can ever replace it:

```yaml
    read_only: [summary, description, due, priority, location, categories, start]
```

The exposure is real and not hypothetical. Verified against shipping clients:
Apple Reminders normalizes `DTSTART` to match `DUE` on an all-day to-do;
Thunderbird sends rich-text notes whose formatting is flattened on the way in;
both rebuild the whole VTODO on every edit, so a field the client models poorly
is rewritten even when the user only touched something else. None of that is
malice — it is what a foreign data model does to yours.

**This is orthogonal to ACL.** They answer different questions and compose:

| | Question | Granularity | Answer to the client |
|---|---|---|---|
| ACL (`acl.yaml`) | *May this principal write at all?* | per type, per op | `403`, write refused |
| `read_only:` | *May a CalDAV client own this field?* | per mapped field | `2xx`, field discarded |

So ACL is the right tool for "this user may not change tasks" — it refuses the
whole write and audits it. `read_only:` is the right tool for "no CalDAV client,
however well-behaved, gets to own the title" — the write still succeeds for the
fields a client legitimately owns.

**Neither refuses the write on the wire.** A denied CalDAV update answers `2xx`
carrying the entity **as it actually stands**, with the `ETag` suppressed — the
same shape `read_only:` uses, and the same one Apple's CalendarServer uses for
VTODO. The write really was refused (the entity is untouched and an audit row
records it); the client is simply told the truth in the representation rather
than through a status code, so it re-reads and the user watches their edit
revert.

This is deliberate, and it is about clients we do not control. Every honest
refusal code leaves a real client broken: `403` makes Thunderbird keep the
rejected edit forever, `412`/`409` loops it through a modal it cannot resolve,
and `404` would delete a to-do that still exists. See
[client compatibility](caldav-clients.md#what-a-client-does-when-a-write-is-refused)
for the wire evidence.

The consequence to be aware of: **a CalDAV client cannot be used to probe
permissions.** A refused write is indistinguishable from an accepted one by
status alone. Anything needing an authoritative answer uses the HTTP API, which
still answers `403`.

A denied **create** is the exception — it answers `404`, because there is no
stored entity to serve back and nothing exists at that href. That leaves no
divergent local copy, since the client's item never became a server resource.

```yaml
    read_only: [summary, description, due, priority, location, categories, start]
```

That leaves **completion** as the only writable field, which is the useful
minimum: check-off works, and every richer edit belongs in the web app.

The names are the **mapping's** names — the YAML keys above — not entity
property names and not iCalendar property names. So repointing `due: deadline`
at another property keeps the lock working without editing the list. One name
covers both spellings of a field: `priority` also covers `priority_map`, and
`description` also covers the `body` sentinel.

Accepted names: `summary`, `description`, `due`, `priority`, `location`,
`categories`, `start`, `completion`. A name outside that set is a **startup
error**, not a silently ignored line — a typo'd lock that leaves the field
writable is invisible in exactly the direction that matters. Naming a field the
collection does not map is reported for the same reason.

What the user sees: the write **succeeds** (the writable fields apply) and the
response carries the entity's real values, so the client's next poll shows the
server's version. Rejecting the whole PUT would be worse — a client sends the
entire VTODO on every edit, so refusing it would also discard the check-off the
user actually meant.

**Whether the client shows the revert is up to the client**, and they differ.
Apple Reminders reverts a refused edit within seconds. Thunderbird has been
observed keeping an optimistic local copy that survives an app restart. So a
discarded field is guaranteed not to reach rela — it is *not* guaranteed to look
discarded in the app. Tell users which fields are server-owned rather than
relying on the UI to show it.

**Creation is exempt.** `read_only` protects a *stored* value, and a new to-do
has none; dropping SUMMARY on a create would produce a titleless entity, since
that is the one field every client sends. So a client sets a read-only field on
the way in, and can never change it afterwards.

To refuse client-created entries altogether, use ACL — withhold `create` on the
type. (Leaving `defaults:` unable to satisfy a required property also blocks it,
but as a side effect rather than a stated rule, and config validation rejects
that configuration at startup precisely because it is unintentional.)

A caveat on "exactly once": a client's create is often a create **followed
immediately by an update** — Apple Reminders was observed doing this, PUTting
the new to-do and then writing again to stamp its own bookkeeping. Both halves
are the same user gesture. The create carries the client's values through; the
follow-up is subject to `read_only:` like any other edit. In practice the two
agree, so nothing visible happens — but an ACL granting `create` without
`update` makes that follow-up fail permanently, and Reminders retries it on
every sync cycle forever. Grant both or neither.

**Do not filter out completed items with `where:`.** A `status != done` clause
breaks the checkbox: rela records the completion, the entity then stops matching
the filter, the resource disappears from the collection, and the client restores
its local copy *unchecked*. A filtered-out resource is indistinguishable from a
deleted one.

RFC 4791 §7.8.9 makes hiding completed to-dos the **client's** job, and
Reminders already does it — they move to a "Completed" section. rela rejects
this configuration at startup rather than letting you discover it as a
mysteriously reverting checkbox.

`on_delete` decides what a client-side delete means. `set:` performs a soft
delete (shown above: mark it cancelled, keeping the entity and its relations);
omit it entirely and deletion is refused with 403.

## 2. Configure Pratique

Enable the PAT class and give it a low-impact capability for to-do access:

```yaml
tls:
  cert_file: "/etc/pratique/cert.pem"
  key_file: "/etc/pratique/key.pem"
  # ...or ACME instead (mutually exclusive with cert_file/key_file):
  # auto:
  #   enabled: true
  #   domains: ["rela.example.com"]
  #   email: "ops@example.com"
  #   storage_dir: /var/lib/pratique/certs   # MUST persist across restarts

long_lived_tokens:
  enabled: true
  low_impact_capabilities:
    - id: "todo:read"
      label: "To-dos"
      description: "Read and update your to-do items"
  idle_expiry: 90d

upstream:
  target: "localhost:8899"      # rela-server
  audience: "app://rela"
```

`idle_expiry` only expires tokens that go **unused** — a working calendar
poller resets the clock on every request, so it never expires out from under a
configured client.

If you use ACME, `storage_dir` must be persistent and backed up: losing it
forces re-issuance, which can hit Let's Encrypt's rate limits.

## 3. Run rela behind it

```bash
rela-server --project /srv/rela \
  --port 8899 --bind 0.0.0.0 \
  --jwt-issuer   https://rela.example.com \
  --jwt-audience app://rela \
  --jwt-jwks-url https://rela.example.com/.well-known/pratique/jwks.json
```

`--jwt-header` already defaults to `X-Auth-Assertion`, so it needs no flag.

**On `--bind 0.0.0.0`:** behind a proxy the forwarded `Host` must be accepted,
and binding to all interfaces is currently the only way to disable the host
check (it sets the allowlist to nil). This is blunter than it should be — a
narrower `--allowed-host` is tracked separately, since it affects every proxied
deployment, not just CalDAV. Until then, ensure rela's port is not directly
reachable from outside the host.

## 4. Issue a credential

PATs are minted in Pratique's web UI under **account → tokens**. There is
deliberately no admin CLI: issuance requires an authenticated session, so an
operator cannot mint a token on a user's behalf.

**The plaintext is shown once.** Copy it before closing the page.

## 5. Add the account on macOS

**System Settings → Internet Accounts → Add Account → Other Account → CalDAV
account**

| Field | Value |
|---|---|
| Account Type | **Manual** |
| Username | anything — it is display-only |
| Password | **the PAT** |
| Server Address | `https://rela.example.com` — the `https://` prefix is **required** |

Then make sure **Reminders** is toggled on for the account.

The list appears in Reminders under the name from `meta.name`.

### When it fails

**macOS reports every setup failure as "account name or password verification
failed"**, whatever the actual cause — a missing `https://` prefix, an
unreachable host, an untrusted certificate and a genuinely wrong token all
produce that one message. Read the server log, not the dialog.

Discovery happens over three requests, and a failure in any of them surfaces as
that same message:

1. `GET /.well-known/caldav` → redirect to the CalDAV root (RFC 6764)
2. `PROPFIND /` → the `current-user-principal` href
3. `PROPFIND` on the principal → the calendar home set

If the account connects but shows **no lists**, step 2 is the one to check.

## Other clients

The endpoint is standard CalDAV (RFC 4791) serving `VTODO`, so any compliant
to-do client should work. **Apple Reminders** and **Mozilla Thunderbird** are
verified end-to-end against a live deployment.

[CalDAV client compatibility](caldav-clients.md) has a per-client table —
platforms, VTODO support, rich-text handling — with sources.

Two things worth knowing from it. Collections must advertise
`<C:comp name="VTODO"/>`, which rela does: **Tasks.org silently hides** a
collection that does not, with no error message.

And **give Thunderbird the calendar HOME SET, not a collection URL**:

```text
https://<host>/api/v1/_caldav/principal/calendars/
```

Pointed at the home set it walks the discovery chain (`PROPFIND` the principal,
then the home set) and offers every collection it finds in a subscribe picker,
already-subscribed ones greyed out. Pointed at a single collection URL it treats
that as the whole calendar and never looks for siblings — which is why an
account set up that way silently misses collections added later.

Verified on the wire against Thunderbird 153 (2026-08-18): given the home set it
discovered a static collection and two graph-driven ones, and subscribed to both
new ones in a single step.

Clients that support the CalendarServer `getctag` extension get a cheap
collection poll: one property fetch tells them whether anything changed, so an
unchanged collection costs a single small request instead of a full enumeration.

## Deletion semantics

Deleting a to-do in the client applies the collection's `on_delete` rule.

Deleting an entity **in rela** — in the SPA, the CLI, or via `git pull` — is
treated as authoritative. A client that has not synced will eventually PUT its
cached copy back; that write is refused with `404`, so the client drops its
local copy rather than resurrecting the entity. This holds regardless of how the
entity was deleted, including deletions made while the server was stopped.

## Constraints

- **CalDAV is unavailable in `rela-desktop`.** It opens no network listener —
  Wails drives the router in-process — so there is no socket for a client to
  reach.
- **rela performs no CalDAV-specific authentication or transport check.** The
  TLS requirement is a property of the client↔Pratique hop, which rela cannot
  observe; the JWT gate failing closed on a missing or invalid assertion is the
  actual protection.
- **Single-writer alias state on the filesystem backend.** The CalDAV↔rela
  resource alias table is guarded by an in-process mutex, so one rela process
  should serve CalDAV for a given project.
- **Aliases are per-principal.** The table is keyed by
  `(principal, collection, href)`. An href is a client's own bookkeeping — Apple
  mints a bare UUID — so two identities syncing the same collection keep
  separate namespaces and neither sees the other's hrefs. This is also what lets
  a future principal-bound `where:` filter work: when a clause resolves to a
  different member set per principal, one config key is no longer one
  collection, and the alias table has to be able to say so.
