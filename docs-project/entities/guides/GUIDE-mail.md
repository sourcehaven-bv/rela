---
audience: intermediate
id: GUIDE-mail
order: 12
status: published
summary: Configure outbound email — SMTP, branding, and what best-effort delivery means
title: Outbound Mail
type: guide
---

Rela can send email. This guide covers configuring a mail transport and what to
expect from it.

> **Nothing triggers mail yet.** This release ships the transport — configuring
> a mail server, branding, and delivery behaviour — but no way to *send* a
> message. Scheduled digests and change notifications land next; see
> [What you cannot do yet](#what-you-cannot-do-yet) before you set this up
> expecting a daily reminder.

> **Read the delivery guarantee section before you rely on this.** Delivery is
> best-effort by design, and the failure mode is specific enough to be worth
> understanding up front.

## Quick Start

Create `.rela/mail.yaml` in your project:

```yaml
transport: smtp
host: smtp.example.com
port: 587
username: relay@example.com
from: rela@example.com
from_name: "Rela"
base_url: https://rela.example.com
```

Then put the password in `.rela/secrets.yaml`:

```yaml
smtp_password: your-smtp-password
```

If `.rela/mail.yaml` does not exist, mail is simply off. Every other command works
exactly as before — an absent config is a normal state, not an error.

## Configuration

| Key | Required | Description |
| --- | --- | --- |
| `transport` | yes | `smtp` or `memory` |
| `host` | for smtp | SMTP server hostname. A bare hostname, not a URL |
| `port` | no | Defaults to `587` |
| `username` | no | Omit for a relay that accepts unauthenticated submission |
| `password_env` | no | **Name of an environment variable** holding the password. Optional — see below |
| `from` | yes | Envelope and header sender |
| `from_name` | no | Display name for the sender |
| `timeout_seconds` | no | Per-send timeout. Defaults to `30` |
| `base_url` | no | Public app URL, used to resolve links in mail |

### The password lives with your other secrets

Put it in `.rela/secrets.yaml`, alongside every other credential rela uses:

```yaml
# .rela/secrets.yaml
smtp_password: your-smtp-password
jira_api_key: sk-abc123
```

That is the same file Lua scripts read. An SMTP password is no different in kind
from an API token, so it goes in the same place rather than in a mechanism unique
to mail.

**Or use an environment variable.** If your deployment injects credentials as
environment variables — containers, systemd units — name the variable instead:

```yaml
# .rela/mail.yaml
password_env: RELA_SMTP_PASSWORD
```

`secrets.yaml` takes precedence when both are present.

Either way, **the password never goes in `mail.yaml`**. Writing a literal
`password:` key there is refused at load with an error pointing you at the
alternatives. This is the rule that keeps the database DSN env-only too: a
secret must never reach a command line, where it lands in `ps` output and shell
history. `.rela/` is gitignored by convention, but a config file is still
something people paste into bug reports.

### TLS is mandatory

Connections use STARTTLS, and rela **refuses to send** to a server that does not offer
it rather than falling back to an unencrypted connection. Certificates are verified;
there is deliberately no option to skip verification.

If you are running a mail server that only speaks plaintext, put a TLS-terminating
relay in front of it rather than asking rela to downgrade.

## Delivery is best-effort

Mail is queued in memory and delivered by a background worker. This keeps sends off
the request path: a save is never blocked waiting on a mail server, and a mail server
that is down degrades to a logged delivery failure instead of a failed write.

The cost is that **undelivered mail does not survive a restart**, and the details
matter:

- In **`rela-server`** there is no shutdown handler, so there is no drain at all —
  every message still queued when the process stops is lost.
- Where shutdown does run — CLI commands, the desktop app, tenant eviction — the
  worker gets a bounded window (10s) to finish in-flight sends before it is
  abandoned.
- A failed send is retried with exponential backoff, up to 5 attempts, and then
  given up on with an error in the log.
- **One bad message delays the good ones.** Delivery is sequential, so an
  undeliverable message — a typo'd address, most likely — holds up everything
  behind it for the length of its retry ladder, about 30 seconds. A batch of bad
  addresses adds up.
- If the queue fills (128 messages), further sends are **rejected with an error**
  rather than silently dropped. A full queue means the mail server is unreachable
  and a backlog is building — or that a stalled message is holding the line.
- If a shutdown drain times out (10s), the send in flight is left to finish on its
  own and its connection is not returned. On a server hosting many projects, where
  eviction happens routinely, those connections accumulate against your provider.

So: **mail here is notification, never a system of record.** Do not build anything
that assumes a message arrived. If you need delivery guarantees, send to a real queue
or a transactional mail provider that provides them.

A durable queue with swappable backends is planned; when it lands, this caveat
narrows considerably.

## What you cannot do yet

The transport works, but nothing in this release calls it. Concretely, there is
**no way to schedule a daily digest or send on a change** — those arrive in the
next two pieces of work:

**Declarative mail** will let you describe the content once and trigger it from
a schedule:

```yaml
# planned — not available yet
mail_templates:
  overdue_digest:
    subject: "Tasks due {{today}}"
    sections:
      - title: "Overdue"
        entity_type: task
        where: ["status != done", "due < today"]
        columns: [title, due, owner]
```

```yaml
# schedules.yaml — planned
tasks:
  - name: daily-digest
    template: overdue_digest
    every: day
    run_as: system:digest
```

The same templates will be callable from an automation, so a status change can
send a notification.

**Sending from Lua** will add a `mail.send` binding for anything the declarative
form does not cover.

Until then, configuring `.rela/mail.yaml` is useful only to verify your mail
server works — point it at a local capture tool such as
[Mailpit](https://mailpit.axllent.org/) and confirm the connection, TLS and
credentials are right before the triggers land.

## Local development

Use the `memory` transport to exercise mail without a mail server — and without any
risk of mailing real people:

```yaml
transport: memory
from: rela@example.com
```

Messages are recorded in process instead of being sent. This is the recommended
setting for `just dev` and for any test environment pointed at production-like data.

## Branding

Mail reuses your existing theme rather than introducing a second one. The accent
colour comes from your palette, and the operator logo is embedded in the message.

Two constraints worth knowing:

- **The logo is embedded, not linked.** The logo endpoint sits behind authentication,
  so a mail client could never fetch it — the bytes travel with the message.
- **SVG logos are skipped.** SVG has near-zero support in mail clients and is an
  active-content format. If your logo is an SVG, mail renders without it; upload a
  PNG if you want it to appear.

Messages are sent as `multipart/alternative` — an HTML part plus a plain-text
alternative — using a table-based layout that renders correctly in Outlook, Gmail and
Apple Mail.

## Multi-tenant deployments

Each project gets its own mail worker and its own connection to your mail server. A
server hosting many projects against one mail provider will open one connection per
project; check that against your provider's connection limits.

## Troubleshooting

**Mail is off and I don't know why.** An absent `.rela/mail.yaml` is silent by
design. An *invalid* one logs a warning at startup naming the problem — check the
server log for `mail: disabled`.

**Sends fail with a TLS error.** The server must offer STARTTLS with a valid
certificate. A self-signed certificate will be rejected; use a real one, or terminate
TLS at a relay.

**Nothing arrives and there are no errors.** Check the log for `mail: delivery
failed`. If the process restarted while messages were queued, they are gone — see
the delivery section above.
