---
id: DOCS-MAIL01
type: docs-checklist
title: 'Docs: Mail foundation — SMTP + in-memory transports, best-effort outbox, branded rendering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

Both new packages carry package docs, and each documents the thing that would
otherwise be undone by a well-meaning edit:

- **`internal/mailrender`** — the pipeline order (`markdown → goldmark →
  bluemonday on the CONTENT ONLY → trusted template → douceur inline LAST`) is
  documented as a *security property*, with both verified library behaviours
  spelled out: bluemonday strips `style` attributes (so sanitizing the assembled
  document ships unstyled mail and also strips the `cellpadding`/`border`/`role`
  and `cid:` sources email needs), and douceur validates no CSS values (so
  nothing may sanitize after it). Reversing either is a silent downgrade — mail
  still sends, just unstyled or unsafe — which is exactly why prose is the only
  thing standing between a future edit and the bug.
- **`text.go`** — documents *why* it walks goldmark's AST rather than
  pattern-matching, naming the three bugs the hand-rolled version had
  (unescape-after-strip re-materializing markup, `ReplaceAll` turning `5*3*2`
  into `532`, first-`)` truncating URLs). Someone tempted to "simplify" it back
  to regexes reads what that costs.
- **`internal/mail`** — the package doc states the best-effort contract in
  specifics rather than euphemism, including that `rela-server` has no signal
  handler so there is no drain at all. `Outbox` documents head-of-line blocking
  with the ~30s arithmetic; `Stop` documents that a timed-out drain leaves a
  goroutine holding a connection.
- **`Sender`** — documents that implementations must not mutate the message,
  naming the retry path that would otherwise send something different on attempt
  two. `mailtest` has a conformance clause for it.
- **`Message.RenderedFor`** — documents that this ticket enforces nothing with
  it and that it exists as the seam TKT-U2R7GU attaches its ACL gate to, so a
  reader does not mistake an unused field for a forgotten one.

## Project Documentation

- [x] `CLAUDE.md` updated with new patterns
- [x] Package table updated

`CLAUDE.md` gains a mail entry under the rules covering the three things a
future contributor could plausibly get wrong: the pipeline order, that
header-injection validation is rela's job rather than the SMTP library's (go-mail
rejects CR/LF in addresses but *accepts* it in a subject), and that the outbox is
not a durable queue. Both packages are in the package table.

## External Documentation

- [x] User-facing guide written
- [x] Generated docs regenerated and committed

`docs-project/entities/guides/GUIDE-mail.md` → `docs/mail.md` (source-edited,
then `just docs`; `just docs-check` passes).

Written for an operator, and deliberately front-loads the delivery caveat rather
than burying it: a "Delivery is best-effort" section states that `rela-server`
loses queued mail on every restart with no drain, that one bad address stalls
the queue ~30s, that a full queue rejects rather than drops, and that a
timed-out drain leaks a connection. The guide says plainly: *mail here is
notification, never a system of record.*

Also covers `password_env` (and why the password is never in the file),
mandatory STARTTLS with no skip-verify option, the `memory` transport for local
dev, branding including why the logo is CID-embedded and why SVG is skipped, the
per-tenant connection cost, and a troubleshooting section for the three failure
modes an operator will actually hit.

## Verification

- [x] Docs match the shipped behaviour

Every claim in the guide corresponds to a test: the absent-config path
(`TestLoadConfig_Missing`, `TestStartMail_NotConfigured`), STARTTLS refusal
(`TestSMTP_RefusesPlaintextDowngrade`), queue-full rejection
(`TestOutbox_FullReturnsError`), loss on stop (`TestOutbox_LosesPendingOnStop`),
and raster-only logos (`TestMessage_RejectsNonRasterInlineImage`).
