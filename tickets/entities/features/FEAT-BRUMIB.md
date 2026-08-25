---
id: FEAT-BRUMIB
type: feature
title: 'Outbound email: pluggable transport, async outbox, branded HTML rendering, and declarative mails'
description: Let rela send email from three trigger surfaces (scheduler, automations, Lua) with pretty-by-default HTML rendered in Go, a pluggable SMTP/HTTP-API transport, and an async outbox so writes never block on a mail server.
status: proposed
---

rela can read, validate and present a graph but has no way to *tell anyone*
about it. The recurring operator need is notification: a daily digest of overdue
tasks, an upcoming-events mail, a meeting reminder carrying its agenda body, and
"this entity changed, go look" when an automation fires.

Today the only path is a Lua script on `schedules.yaml` calling `http.post` at
some webhook — no email, no HTML, no retry, and every secret in `secrets.yaml`
handed to the script.

**Four capabilities, split across two tickets.**

1. **Pluggable transport.** A `mail.Sender` interface with two real implementations —
SMTP (STARTTLS) and an HTTP JSON API — chosen by `.rela/mail.yaml`. Many hosts
block outbound :587, so an HTTP transport is not a nicety; it is the only option
on those hosts. Two implementations from day one keep the seam honest rather
than theoretical (the `ai.Provider` shape).

2. **Async outbox.** Automations run *inside the write path*. A synchronous SMTP dial
there would block a user's save on a remote server and could double-send on
retry. Mail is enqueued transactionally and delivered by a background worker
with backoff and an idempotency key.

3. **Branded HTML rendering.** Markdown → sanitized HTML → table-based email layout with
CSS inlined, plus a `text/plain` alternative. Operator supplies accent colour
and logo. No Node: MJML's *output* is hand-ported into Go templates, not MJML
itself.

4. **Declarative mails.** A content declaration (subject, intro, sections of
entity_type + where + columns) that covers the common cases without Lua, reusing
the `feeds:`/`lists:` vocabulary operators already know. Triggers stay separate
from content because the scheduler is context-free and an automation always
carries an entity.

**Security posture.** Mail renders entity content *to an inbox*, outside every
read gate — it is an exfiltration surface. Every send resolves through the
triggering principal's visibility wrapper, and per-recipient digests render as
that recipient. Credentials live in `.rela/`, never on the wire and never on a
command line.
