---
id: IMPL-U2YD0E
type: implementation-checklist
title: 'Implementation: Mail foundation: Sender interface (SMTP + in-memory), best-effort outbox, branded HTML rendering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Shipped: `internal/mailrender` (leaf: markdown → sanitized, CSS-inlined, branded
HTML + text/plain), `internal/mail` (Sender, SMTP via go-mail, memory transport,
`.rela/mail.yaml`, best-effort outbox), `internal/mail/mailtest` (transport
conformance suite), wiring in `appbuild.assemble`/`Close`, arch-lint + coverage
registration, `docs-project/entities/guides/GUIDE-mail.md`.

Integration, not just units: the conformance suite runs the **real SMTP client
against a real SMTP server over real STARTTLS with real certificate
verification**, and the manual harness drives render → outbox → wire end to end.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`mailtest.validMessage()` / `testMessage()` / `base()` are the builders; each
case mutates only the field under test. Table-driven with `t.Run` subtests
throughout. `mailtest.RunAll` uses the map-driven shape from `statetest`, so
cases cannot develop an order dependency.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

End-to-end harness (`internal/mail/manual_e2e_test.go`, `//go:build mailmanual`,
excluded from the default build — not "temporary code removed before merge"):
renders a realistic digest with hostile content embedded, sends it through the
outbox over SMTP+STARTTLS, captures the wire bytes.

Received message structure (`/tmp/mailmanual/received.eml`):

```
Subject: =?UTF-8?q?3_tasks_need_you_=E2=80=94_T=C3=A2ches_en_retard?=
From: "Rela" <rela@example.com>
Content-Type: multipart/related;
  └ multipart/alternative
      ├ text/html;  quoted-printable
      └ text/plain; quoted-printable
  └ image/png; base64   (Content-ID: <logo@rela>)
```

Decoded HTML part, hostile input `<script>alert(1)</script>` in a table cell:

| Check | Result |
| --- | --- |
| `<script>` executable | 0 |
| `onerror` | 0 |
| `javascript:` | 0 |
| `&lt;script&gt;` (escaped, inert) | 1 — correct: the cell's *value* was that text |
| `style="` (inlining survived) | 30 |
| `[if mso]` (Outlook fallbacks) | 2 |
| `cid:logo@rela` | 1 |
| SMTP password in message bytes | 0 |

Per acceptance criterion:

| AC | Evidence |
| --- | --- |
| 1 config absent → off | `TestLoadConfig_Missing`, `TestStartMail_NotConfigured`; invalid config disables + logs (`TestStartMail_InvalidConfigDoesNotFailBoot`) rather than failing boot |
| 2 SMTP + STARTTLS | `TestSMTP_DeliversOverSTARTTLS` (real cert, AUTH creds observed server-side); `TestSMTP_RefusesPlaintextDowngrade` asserts refusal, nothing transmitted |
| 3 memory records | `TestMemorySender`, `TestMemorySender_RingBuffer` (bounded retention, lifetime count) |
| 4 conformance | `TestConformance_Memory` + `TestConformance_SMTP`, 11 shared clauses |
| 5 render golden | `TestRender_Golden`, `testdata/digest.golden.{html,txt}` |
| 6 sanitize | `TestRender_SanitizesUntrustedContent` |
| 6a ordering guard | `TestRender_KeepsInlineStyles` — fails if the assembled document is sanitized |
| 6b mso survives | `TestRender_PreservesMSOConditionals` |
| 6c CSS injection | `TestRender_NoDangerousCSSReachesStyleAttributes`, asserted post-inline |
| 6d palette allowlist | `TestValidatePalette` (15 cases), `TestNew_RejectsBadPalette` |
| 7 text alternative | `TestRender_AlwaysHasTextPart` (5 message shapes) |
| 8 enqueue non-blocking | `TestOutbox_EnqueueDoesNotDial` against a permanently blocked transport |
| 9 retry no-dup | `TestOutbox_RetriesWithoutDuplicating` — 2 failures, 1 delivery, 3 attempts |
| 9a CRLF subject | `TestMessage_Validate` + `mailtest` clause, asserting rejection at the boundary |
| 9b bounded stop | `TestOutbox_StopIsBoundedAndIdempotent` |
| 10 loss on restart | `TestOutbox_LosesPendingOnStop` — pins the limit as intended behaviour |
| 11 no credential leak | `TestSMTP_CredentialNeverInError`; 0 occurrences in wire bytes |
| 12 arch-lint | `just arch-lint` → "OK - No warnings found" |

Edge cases verified: unicode subject/body round-trip
(`TestRender_UnicodeSubjectAndBody`, encoded-word confirmed on the wire); empty
section renders a note not a header-only table; relative links resolve against
BaseURL in **both** parts (`TestRender_TextPartResolvesLinks` — found and fixed
during manual verification); unsafe schemes dropped; outbox full returns
`ErrOutboxFull`; poison message does not wedge the queue
(`TestOutbox_GivesUpAfterMaxAttempts`); concurrent enqueue under `-race`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `ai/config.go` (sentinel + `Validate` + env-named credential),
`pgstore/sweep.go` (detached ctx, `done` channel, nil-safe idempotent stop,
`err != nil && ctx.Err() == nil`), `appbuild/datamigration.go` (`startX() (stop
func())`, never fails boot), `storetest`/`statetest` (conformance kit),
`internal/mcp/golden_test.go` (`UPDATE_GOLDEN=1`), `calfeed` (leaf contract).

DRY: `validateHeaderValue` is the single header-safety check used by config,
message and inline images. `safeHref` is shared by the HTML and text renderers —
the divergence between them was a real bug this consolidation fixed. Not
extracted: the two `template.HTML` conversion sites, which are three lines each
and clearer inline.

Security: pipeline order documented as load-bearing with a regression test;
palette allowlisted; CR/LF/NUL rejected at the boundary; STARTTLS explicit with
no skip-verify knob; credential env-named and redacted; SVG logos refused.

**Two bugs found by the tests during implementation**, both fixed:
`html/template` silently strips HTML comments (deleting the Outlook fallbacks),
and the text part passed raw HTML through unsanitized. A third — a goroutine leak
in the SMTP fake causing an intermittent goleak failure — is RR-1UEMS9.
