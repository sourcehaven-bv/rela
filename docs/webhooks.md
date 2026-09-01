# Declarative webhook routes

Map an inbound HTTP request onto entity writes from `data-entry.yaml`, without
writing Lua. The common integration shapes — a monitoring alert, a form post, an
upstream event — are all the same small pipeline.

Declared under `webhooks:`; **the hook id is the URL segment**, so
`webhooks: { icinga-alert: ... }` serves `POST /hooks/icinga-alert`. There is no
`path:` key.

## Producer authentication is NOT rela's job

**rela does not authenticate the sender.** There is no HMAC verifier, no shared
secret and no per-endpoint token, and none is planned. The fronting proxy
(Pratique, oauth2-proxy, or your ingress) terminates producer authentication and
hands rela an ACL-bounded request.

If you expose `/hooks/*` directly to the internet with no proxy, anyone who can
reach it can drive these pipelines. That is a deployment decision, not a gap to
work around in config.

## Three workflows

Uniqueness is a property of *your* schema, not something the webhook layer
assumes. Which workflow you get depends on which blocks you write:

| `find:` | `create_if_missing:` | Workflow |
|---------|----------------------|----------|
| absent  | present | **Always create** — every delivery makes a new entity |
| present | present | **Find-or-create** — look up, create when absent, then mutate |
| present | absent  | **Find and update only** — a miss is a no-op |

`find:` is optional, and a `match:` key is required only when
`create_if_missing:` is present. Without one, find-or-create could not identify
the entity the previous delivery created, so every alert would mint a duplicate.

### Always create

```yaml
webhooks:
  contact-form:
    create_if_missing:
      type: enquiry
      properties:
        title: "{{body.subject}}"
        email: "{{body.email}}"
    respond: { status: 202 }
```

### Find-or-create

```yaml
webhooks:
  icinga-alert:
    find:
      type: incident
      match: [host, service]
    create_if_missing:
      template: incident
      properties:
        title: "{{body.host}}/{{body.service}}"
        host: "{{body.host}}"
        service: "{{body.service}}"
        status: open
    then:
      - append_section:
          section: Notifications
          content: "- {{now}} **{{body.state}}** — {{body.output}}"
    respond: { status: 200 }
```

### Find and update only

```yaml
webhooks:
  resolve-incident:
    find:
      type: incident
      match: [host, service]
    then:
      - set:
          status: resolved
```

A delivery matching nothing answers `200` with `{"action":"no_match"}`. It is
not an error — nothing to update is a legitimate outcome.

## Identity: a computed, unique property

You name the source fields; **rela derives the key**. Declare the key as a
`computed:` property on the entity, not as a hash in the webhook config:

```yaml
# schema.yaml
incident:
  properties:
    alert_key:
      type: string
      unique: true
      computed: sha256(entity.host .. "/" .. entity.service)
```

The key belongs to the entity, so *every* writer — webhook, Lua, CLI, sync —
derives the same identity. A webhook-local hash would leave the invariant
unenforced everywhere else.

Computed values are materialized on **write**, so `find:` matches on stored
values. The webhook therefore sets the SOURCE fields (`host`, `service`) and
rela derives the key — not the reverse.

### `sha256()` returns lowercase hex

`sha256(s)` in a `computed:` expression returns the **64-character lowercase hex**
digest. This encoding is effectively permanent: the value is stored, indexed and
usually `unique:`, so changing it would invalidate every key already persisted.
Hex matches `crypto.sha256_hex` in Lua and the content-hash key Icinga DB uses
for the same purpose.

## Concurrency: what is and is not guaranteed

The pipeline detects conflicts rather than locking, and retries a bounded number
of times (4 attempts) before answering `409`.

**Be precise about the guarantee you get, because it differs by tier:**

| Situation | Guarantee |
|---|---|
| Concurrent creates, `unique:` declared, **PostgreSQL** | Race-free. The derived unique index is atomic; the loser re-finds and updates. |
| Concurrent creates, `unique:` declared, fs / sqlite | Safe in practice — those tiers are single-writer — but the uniqueness check is a scan, not a constraint. |
| Concurrent creates, **no** `unique:` | Can duplicate under concurrent delivery. The vocabulary offers the safety; it does not mandate it. |
| Concurrent `append_section`, one rela process | Safe. Deliveries serialize on the process write lock and each attempt re-reads. |
| Concurrent `append_section`, **several rela processes on one database** | **An append can be lost.** Nothing in this path is a compare-and-swap across processes. |

That last row is a real limitation, not a theoretical one — it is reproduced by
`TestWebhookConflict_CrossProcessAppendsCanBeLost`. If you run multiple
`rela-server` processes against one PostgreSQL database and rely on
`append_section`, be aware that a simultaneous delivery to the *same* entity from
two processes can drop one line. Property `set:` steps are unaffected (they merge
server-side). A server-side append mode is the planned fix.

## Delivery loss is the sender's to solve

rela does not persist-then-process. A rela restart or outage mid-delivery loses
that delivery, and a producer that does not retry (Icinga executes a notification
command exactly once and never re-queues) will lose the alert whatever rela does.

Close this sender-side: a notification command is just a script, so have it POST
with retry (`curl --retry 5 --retry-connrefused`, or a wrapper). For guaranteed
capture, poll the producer's API/event stream rather than relying on push. This
matches rela's existing stance that mail is "notification, never a system of
record".

## Template vocabulary

| Reference | Resolves to |
|---|---|
| `{{body.<path>}}` | A field of the parsed body; dotted paths walk nested objects |
| `{{query.<name>}}` | A query-string parameter |
| `{{header.<name>}}` | An **allowlisted** request header (case-insensitive) |
| `{{now}}` | Delivery timestamp, RFC 3339 UTC |
| `{{today}}` | Delivery date, `YYYY-MM-DD` |

An unresolved reference becomes the **empty string**, not the literal text. A
stored `{{body.host}}` would be a silent corruption that looks like a template
bug forever; empty is visibly missing and cannot be mistaken for data.

### Body encodings

Bodies may be JSON (the default) or `application/x-www-form-urlencoded`. The
media type is read from `Content-Type`; parameters and case are ignored, so
`application/json; charset=utf-8` and `APPLICATION/JSON` both work. Anything
unrecognised — including a missing `Content-Type` — is parsed as JSON.

Two differences follow from a form body being a flat list of string pairs:

- **Nested paths do not resolve.** `{{body.alert.host}}` addresses nested JSON;
  a form has no nesting, so with a form body it resolves to the empty string.
  Send JSON if you need structure.
- **A repeated key keeps the FIRST value.** `tag=a&tag=b` yields `a`; `b` is
  dropped. One key addresses one value, and a flat body cannot represent both.

An empty body is valid — useful for a hook driven entirely by `{{query.*}}`.
An unparseable body of either kind is a `400` and writes nothing.

## Headers are an allowlist

Headers are exposed to templates only if you list them:

```yaml
webhooks:
  icinga-alert:
    headers: [X-Delivery-Id]
```

Anything not listed resolves to empty. This is deliberate: headers carry session
cookies, bearer tokens and proxy-injected identity assertions, and a template
that could reach any header would let a hook persist a credential into entity
content — where it is then served back on every read.

Credential-bearing and identity-asserting names (`Authorization`, `Cookie`,
`Proxy-Authorization`, the `X-Forwarded-*` / `X-Auth-Request-*` family,
`X-Remote-User`) are **refused at config load** whatever you write.

## Body size cap

Inbound bodies are capped at **1 MiB** by default; override per hook with
`max_body_bytes:` (ceiling 8 MiB). An oversized body is rejected with `413` and
writes nothing — it is never truncated, because a truncated form body parses
cleanly and would store a quietly-wrong entity.

## `append_section`

```yaml
then:
  - append_section:
      section: Notifications
      content: "- {{now}} {{body.state}}"
```

The line is appended after the last content line of the named section (before the
next heading of the same or higher level). Heading matching is
case-insensitive.

**A missing section is created**, as a new `## <section>` at the end of the body,
rather than erroring. For an alert pipeline an error would discard a delivery the
producer will never resend; an unplanned heading is visible and trivially edited.
It also means a freshly created entity accumulates notifications from the first
delivery without the template and the hook having to be kept in sync.

Steps must be **pure**: a conflict retry re-runs the whole `then:` list.

## Responses

| Status | Meaning |
|---|---|
| `200` (or your `respond.status`, any 2xx) | Delivered. Body: `{"hook","action","entity_id"}` where `action` is `created`, `updated` or `no_match` |
| `400` | Malformed body |
| `404` | No such hook |
| `409` | Lost the conflict retry budget — retrying is reasonable |
| `413` | Body over the cap |
| `500` | Pipeline failure (details in the server log, not the response) |
| `504` | Exceeded the 30s processing timeout |

Errors deliberately do not echo internals: a failure message can carry stored
property values, and the producer is not necessarily entitled to them.

## Validation is at load time

A malformed hook is a **load error**, not a runtime surprise — an unknown entity
type, a match on a property that does not exist, a step with two actions, a
non-2xx `respond.status`, a forbidden header. A webhook that silently misfires
loses data from a producer that does not retry, so failing the load is the safe
direction.

Routes are mounted per configured hook at startup, so **adding or removing a hook
needs a restart**. Changing an existing hook's behaviour (its find, steps or
response) is picked up on config reload.

## ACL

Reads go through the same ACL read path as every other read: gated on the stored
type, `visible:`-hidden fields redacted, and a denied entity reported as a plain
miss. A hook whose principal cannot see the matching entity will therefore
*create* a new one rather than silently updating an invisible row.

Writes go through `entitymanager`, so they are audited like any other write and
attributed to `webhook:<hook-id>`.
