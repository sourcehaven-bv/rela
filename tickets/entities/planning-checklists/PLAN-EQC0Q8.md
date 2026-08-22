---
id: PLAN-EQC0Q8
type: planning-checklist
title: 'Planning: Mail foundation: Sender interface (SMTP + in-memory), best-effort outbox, branded HTML rendering'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IS: `internal/mailrender` (leaf: message model → branded HTML + text/plain bytes);
`internal/mail` (Sender interface, SMTP transport via go-mail, memory transport,
`.rela/mail.yaml` config, best-effort in-process outbox + worker); wiring in
`appbuild.assemble`; docs.

IS NOT: HTTP-API transport, Lua script transport, `mail.send` binding (all
TKT-DS1CR6); `mail_templates:` config, scheduler/automation triggers, recipient
resolution from the graph, per-recipient ACL scoping (all TKT-U2R7GU); durable
queue (IDEA-WIJ2H1); inbound mail, bounce handling, tracking; DKIM (the provider
signs).

Nothing in this ticket sends mail on its own — there is no trigger yet. The
deliverable is a Sender that a later ticket calls. Verification is therefore via
tests plus one temporary dev-only trigger, NOT via a shipped user-facing feature.

**Acceptance Criteria:** the 12 on the ticket. Each is mapped to a concrete test in
the Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the option space was settled in the ticket discussion
(transport survey across 4 providers, MJML-vs-Go-templates, best-effort-vs-durable).
Findings recorded on TKT-332QZY / TKT-DS1CR6 / IDEA-WIJ2H1 rather than a separate
RES entity.

**Existing Solutions:**

*SMTP library — `github.com/wneessen/go-mail` v0.8.1 (CHOSEN).*
`net/smtp` is **frozen** (its own package doc: "not accepting new features") and
provides only the wire protocol — message assembly would be ours: RFC 5322 header
folding, RFC 2047 encoded-words for non-ASCII subjects, quoted-printable, multipart
boundaries. All are easy to get subtly wrong in ways that surface in one client.
go-mail is MIT, actively maintained, and **verified**: `go get` resolves it adding
only `golang.org/x/crypto` and `golang.org/x/text`, both already in the module graph
at *newer* versions (v0.55.0 / v0.41.0 vs its v0.54.0 / v0.40.0). So it costs
**exactly one new module**, zero new transitive dependencies.
Rejected: `emersion/go-message`+`go-smtp` (same maintainer as the in-tree go-ical /
go-webdav, but two modules and more glue for no gain).

*CSS inlining — `github.com/aymerick/douceur/inliner` (CHOSEN).*
Already in go.sum at v0.2.0 (indirect, via glamour → bluemonday → douceur/parser),
so no version negotiation. **Verified working** against the exact use case:
`inliner.Inline(html)` turns a `<style>` block into inline `style=""` attributes and
strips the `<style>`, which is precisely the Outlook/Gmail requirement.
Cost correction: `douceur/inliner` (unlike `douceur/parser`, the part already
linked) pulls `PuerkitoBio/goquery` + `andybalholm/cascadia` — **2 new modules**.
Rejected: `vanng822/go-premailer` — not in go.sum at all, and depends on douceur +
goquery anyway, so strictly more cost for the same result.

*HTML sanitizing — `bluemonday` v1.0.27*, in go.sum (indirect). Promoting it to
direct is a mechanical `go mod tidy` move, no new module.

*Markdown — `goldmark` v1.8.5*, already a direct dep (FEAT-010).

*MJML — REJECTED.* It is a build-time compiler emitting static table HTML, not a
runtime engine. Running it needs Node, which rela (static Go binary) has no
guarantee of; `cmdexec` fails closed, so mail would silently break per host. The
plan copies MJML's *compiled output* into Go templates once and pins with goldens.

*Codebase patterns reused (all verified at the cited lines):*
- `internal/calfeed` — the leaf-package contract `mailrender` copies (pure
  model→bytes, no store/metamodel imports). `.go-arch-lint.yml:59` declares it with
  **no `deps:` entry at all**, which is how a true leaf is expressed.
- `internal/ai/config.go:26-79` — `LoadConfig` shape: `ConfigFile` const,
  `ErrConfigNotFound` sentinel for absent-file, `os.ReadFile` →
  `errors.Is(os.ErrNotExist)` → `yaml.Unmarshal` → `cfg.Validate()`, every error
  wrapping `path`. `Validate` (config.go:83-121) rejects credentials-in-URL and
  query strings; `api_key_env` names an env var read **at call time**
  (openai.go:370-380) so unrelated commands start with it unset.
- `internal/ai/redact.go:11-24` — `redactKey` scrubbing at error/log construction.
- `internal/store/pgstore/sweep.go:121-160` — the worker lifecycle to copy verbatim:
  detached `context.WithCancel(context.Background())`, `done chan struct{}`,
  `defer close(done)` as the first statement of `run`, nil-safe idempotent `stop()`,
  and `err != nil && ctx.Err() == nil` before logging (suppresses the spurious
  cancel-during-work error — essential for a worker mid-send at shutdown).
- `internal/appbuild/datamigration.go:16-80` — `startX(...) (stop func())` returning
  a no-op `func(){}` on every not-configured branch, never an error; torn down in
  `Services.Close` (appbuild.go:1356-1376).
- `internal/scheduler/scheduler.go:79-90` — `WorkspaceProvider`, a **structural**
  consumer-side interface. `appbuild` does not import `scheduler` (verified: no
  match), yet `*Services` satisfies it. This is the pattern that avoids the plimsoll
  problem below.
- `internal/store/storetest/storetest.go:150` + `internal/state/statetest` — the
  conformance-suite shapes for the transport suite.
- `internal/mcp/golden_test.go:51-79` — golden files gated on `UPDATE_GOLDEN=1`
  (env var, not a flag), with nondeterminism normalized **at capture time**
  (regex-replacing minted IDs) rather than by weakening the comparison.
- `internal/store/pgstore/listener_test.go` — `goleak` (already a direct dep) for a
  goroutine-owning package.
- `internal/lua/http.go:36-79` — bounded `http.Transport`. Not needed this ticket
  (no HTTP transport until TKT-DS1CR6) but noted so `mail` does not invent its own.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Two packages, split by dependency weight.*

`internal/mailrender` — a genuine leaf. Input is a plain struct (subject, intro
markdown, sections, a `map[string]string` of colour tokens, optional logo bytes);
output is `(htmlBytes, textBytes, error)`. It imports goldmark, bluemonday, douceur
and **nothing from `internal/`**. Deliberately takes `map[string]string` rather than
`dataentryconfig.ResolvedPalette`, because `dataentryconfig` transitively depends on
`filter`/`git`/`metamodel` — taking the map keeps the leaf genuinely dependency-free
and satisfies AC 12 with the fewest new arch-lint edges.

Pipeline: markdown → goldmark → **bluemonday sanitize** → inject into a table-based
600px Go template with `<!--[if mso]>` fallbacks → `douceur/inliner` → HTML part.
The `text/plain` part is generated from the same model, not from the HTML.
`html.WithUnsafe()` must NOT be used: unlike the existing `simpleMarkdownToHTML`
(dataentry/helpers.go:324-331) whose input is operator-authored schema prose, mail
carries **entity content**, which is untrusted.

`internal/mail` — Sender + config + outbox.

```go
type Sender interface { Send(ctx context.Context, m Message) error }
```

`Message` carries already-rendered parts (the renderer runs upstream), so a transport
never re-renders and can never disagree with another transport about output.

- `smtpSender` wraps go-mail. STARTTLS **required**; an opportunistic downgrade to
  plaintext must fail, not silently proceed.
- `memorySender` records into a bounded ring buffer; exposes a read accessor for
  tests and dev. Two genuine uses (local dev without a mail server; assertions on
  what *would* have been sent) — and it makes the interface exercised rather than
  asserted from day one.
- Config `.rela/mail.yaml` mirroring `ai/config.go` exactly, with `password_env:`.
  `Validate()` rejects a literal password field outright (so the mistake is loud, not
  a silently-leaking config).
- Outbox: in-process channel + bounded buffer, worker goroutine, capped exponential
  backoff, idempotency key. Enqueue never dials. **Best-effort by design** — stated
  in the package doc and operator docs, not merely implied.

*Wiring — and the constraint that shapes it.*
`appbuild.Services` is at **exactly 25/25** exported methods against
`//plimsoll:max-exported-methods=25` (appbuild.go:86 — verified by count). Adding a
`Services.Mail()` accessor would fail the god-object lint and require arguing the cap
upward. Avoided entirely: `internal/mail` declares its own narrow **structural**
provider interface (the `scheduler.WorkspaceProvider` precedent), so nothing is added
to `Services`' public surface and no `appbuild → mail` accessor edge appears. The
worker is started in `assemble` via `startMailWorker(...) (stop func())` following
`startDataMigration`, with a `mailStop` field torn down **first** in `Services.Close`
so in-flight sends drain before the store closes.

*Branding — a constraint found during research.*
The operator logo is stored in `state.KV` under `theme/logo` + `theme/logo.ext`
(dataentry/theme_logo.go:13-14, cap 256 KiB) and served from
`/api/v1/_theme/logo?v=<hash>` — which **is** an `isAPIPath`, hence behind the auth
gate. **An email client cannot fetch it.** So the logo must be embedded as a
**CID attachment** from the stored bytes (go-mail supports inline embeds), not
linked. Colours come from `dataentryconfig.ResolvePalette(...).Light`, whose
`--accent-color` default is `#4772fb` (palette.go:164), flattened to a
`map[string]string` at the wiring site. This reuses the existing theme mechanism
rather than inventing a second one, per the ticket.

**Files to modify:**

New:
- `internal/mailrender/{mailrender.go,template.go,text.go,mailrender_test.go}`
- `internal/mailrender/testdata/*.golden.html`
- `internal/mail/{mail.go,config.go,smtp.go,memory.go,outbox.go,redact.go}` + tests
- `internal/mail/mailtest/mailtest.go` — transport conformance suite
- `internal/appbuild/mail.go` — `startMailWorker`

Modified:
- `go.mod` / `go.sum` — +go-mail; promote bluemonday, douceur to direct
- `internal/appbuild/appbuild.go` — `mailStop` field, `assemble` call, `Close` teardown
- `.go-arch-lint.yml` — declare `mail`, `mailrender`; register `gomail`,
  `bluemonday`, `douceur` vendors; `mail.mayDependOn: [mailrender]`
- `.testcoverage.yml` — floors for the two new packages
- `docs/` — a mail page; the best-effort caveat stated explicitly

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `.rela/mail.yaml` | operator, local disk (gitignored) | `Validate()`: host non-empty, port in range, transport ∈ {smtp, memory} (**allowlist**), reject a literal `password:` key, reject credentials in any URL form | load error naming the file, mail off |
| SMTP password | env var named by `password_env` | read at **call time**, never at load | send fails with a typed error; startup unaffected |
| Entity content → mail body | the graph (untrusted) | goldmark → **bluemonday** sanitize; never `WithUnsafe()` | script/handler stripped |
| Recipient addresses | caller (TKT-U2R7GU) | parse/validate before enqueue; reject CR/LF (**header injection**) | rejected at enqueue |
| Subject / headers | caller | go-mail owns encoded-words; reject embedded CR/LF | rejected at enqueue |
| Logo bytes | `state.KV`, operator-uploaded | already capped at 256 KiB with an ext allowlist upstream | omitted from mail |

**Security-Sensitive Operations:**
- **Credential handling** — password never in YAML, never on a command line (the
  `RELA_DATABASE_URL` invariant), never in logs or errors. A `redactKey` equivalent
  is applied at every error-construction site. AC 11 tests this.
  Note `rela validate` prints validator errors verbatim (cli/validate.go:410), so a
  mail validator must never embed the env **value** in its message.
- **Header injection** — the classic SMTP hole. CR/LF in any address, subject or
  header value is rejected at enqueue rather than sanitized, so a malformed value
  cannot smuggle extra headers.
- **TLS** — STARTTLS required; downgrade refused.
- **HTML sanitization** — mail is an *exfiltration and injection* surface: content
  leaves the ACL perimeter into an inbox. Sanitizing is not optional.
- **Resource bounds** — outbox buffer bounded (drop-with-log at capacity rather than
  unbounded growth); message size capped; logo capped upstream.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| 1 config absent → off | `LoadConfig` on an empty dir returns `ErrConfigNotFound`; `startMailWorker` returns a no-op stop; a CLI command runs normally |
| 2 SMTP + STARTTLS | in-process SMTP fake (net.Listener speaking enough of the protocol); assert AUTH after STARTTLS; a **separate** case asserts a server offering no STARTTLS is refused |
| 3 memory records | send → assert recipients/subject/both parts; assert no dial occurred |
| 4 conformance | `mailtest.RunAll(t, factory)` — both transports registered from external test packages (`mail_test`), storetest style |
| 5 render golden | markdown with h1/h2, link, bold, GFM table → `.golden.html`; `UPDATE_GOLDEN=1` to regenerate; boundary/Message-ID/Date normalized at capture |
| 6 sanitize | `<script>alert(1)</script>` and `<img onerror=...>` in content → absent from HTML part |
| 7 text alternative | every rendered message has non-empty text/plain; a table degrades to readable text |
| 8 enqueue non-blocking | enqueue against a transport that blocks forever returns immediately (with timeout) |
| 9 retry no-dup | transport fails twice then succeeds → exactly one delivery recorded; assert backoff grew |
| 10 loss on restart | enqueue, stop worker with items pending, assert undelivered — **pins best-effort as intended behaviour** |
| 11 no credential leak | set password env to a sentinel; drive config-load, send-failure and validate paths; assert the sentinel appears in no log/error string |
| 12 arch-lint | `just arch-lint` in CI |

**Edge Cases:**
- Empty body, empty subject, zero recipients (reject at enqueue).
- Non-ASCII subject (encoded-words) and non-ASCII body (quoted-printable) — the
  reason go-mail was chosen; assert a UTF-8 subject round-trips.
- Very long single-line content (header folding).
- Markdown with no headers/links (degenerate but valid).
- Content that is *only* an HTML block.
- Outbox at capacity — bounded, drop-with-log, asserted.
- Worker stopped mid-send — `stop()` returns, no goroutine leak (`goleak`).
- Concurrent enqueue from many goroutines under `-race`.
- Logo absent / oversized / unknown ext → render without it, never fail the send.

**Negative Tests:**
- `transport: carrier-pigeon` → load error naming valid values.
- `password:` literal in YAML → load error telling the operator to use `password_env`.
- `password_env` naming an unset variable → send fails with a typed error; **startup
  still succeeds** (the ai precedent).
- CR/LF injected into a recipient or subject → rejected at enqueue.
- SMTP server rejecting AUTH → typed auth error, no credential in the message.
- Golden mismatch → fails with the "review the diff, never regenerate to make it
  green" guidance.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| `Sender` shaped only by in-process transports; TKT-DS1CR6 adds remote ones | `Message` carries pre-rendered parts and `Send` takes a ctx — both remote-friendly. Re-read TKT-DS1CR6's sketch before finalising the signature. |
| **plimsoll cap**: `Services` at 25/25 exported methods | Structural provider interface in `internal/mail`; nothing added to `Services`' public surface. Verified as the pattern `scheduler` already uses. |
| douceur/inliner adds goquery + cascadia (2 modules) | Accepted: the alternative is hand-written CSS inlining, which is worse. Both are widely used and already in the wider Go ecosystem. Flag in PR. |
| Mail lost on restart | **Intended**; documented and pinned by AC 10. Retired by IDEA-WIJ2H1. |
| Rendering regressions across clients | Goldens pin *our* output. They cannot prove real-client rendering — full client-matrix testing is explicitly out of scope; say so rather than implying coverage. |
| Nothing triggers mail in this ticket | Verification is tests + a temporary dev-only trigger, removed before merge. Stated in Understanding so "manual verification" is not silently skipped at implementation. |
| No graceful shutdown in `rela-server` (`main.go:531`, no signal handler) | Out of scope. `Services.Close()` does run for CLI/desktop/tenant-eviction, so the drain path is real there; note the server gap in docs rather than fixing it here. |

**Effort:** M (unchanged).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/` — a new mail page: `.rela/mail.yaml` reference, `password_env`, STARTTLS
      requirement, branding, and the **best-effort delivery caveat stated plainly**
      ("a notification can be silently lost on restart; mail is notification, never a
      system of record"). Source-edited under `docs-project/` since `docs/*.md` are
      generated.
- [x] `CLAUDE.md` — a short entry: mail config lives in `.rela/`, the renderer is a
      leaf, the outbox is best-effort, and where the durable-queue plan lives.
- [ ] `docs/metamodel.md` — N/A (no metamodel change this ticket)
- [ ] `docs/cli-reference.md` — N/A (no new command)
- [ ] `docs/data-entry.md` — N/A (no UI change)
- [ ] `README.md` — N/A (not project-level until the declarative layer ships)

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
