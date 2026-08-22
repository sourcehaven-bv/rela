---
audience: intermediate
id: GUIDE-mail
order: 12
status: published
summary: Configure outbound email — SMTP, branding, and what best-effort delivery means
title: Outbound Mail
type: guide
---

Rela can send email: notification digests, reminders, and "this changed, go look"
messages. This guide covers configuring a mail transport and what to expect from it.

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
password_env: RELA_SMTP_PASSWORD
from: rela@example.com
from_name: "Rela"
base_url: https://rela.example.com
```

Then set the password in the environment:

```bash
export RELA_SMTP_PASSWORD='...'
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
| `password_env` | no | **Name of an environment variable**, never the password |
| `from` | yes | Envelope and header sender |
| `from_name` | no | Display name for the sender |
| `timeout_seconds` | no | Per-send timeout. Defaults to `30` |
| `base_url` | no | Public app URL, used to resolve links in mail |

### The password is never in the config file

`password_env` names an environment variable; the value is read when a message is
sent. Writing a literal `password:` key is refused at load with an error telling you
to use `password_env` instead.

This is the same rule that keeps the database DSN env-only: a secret must never reach
a command line, where it would land in `ps` output and shell history. `.rela/` is
gitignored by convention, but a config file is still something people paste into bug
reports.

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
- If the queue fills (128 messages), further sends are **rejected with an error**
  rather than silently dropped. A full queue means the mail server is unreachable and
  a backlog is building.

So: **mail here is notification, never a system of record.** Do not build anything
that assumes a message arrived. If you need delivery guarantees, send to a real queue
or a transactional mail provider that provides them.

A durable queue with swappable backends is planned; when it lands, this caveat
narrows considerably.

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
