---
id: REV-SV1225
type: review-checklist
title: 'Review: Mail foundation: Sender interface (SMTP + in-memory), best-effort outbox, branded HTML rendering'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Tests run under `-race` (CI's setting) across `internal/mail`,
`internal/mailrender` and `internal/appbuild`. `just arch-lint`, `just plimsoll`
and `just docs-check` all pass. Coverage: `internal/mail` 92.9%,
`internal/mailrender` 92.4%, both floored at 85 — and the floors were verified
*live* by temporarily setting an impossible threshold and confirming the gate
failed, rather than assumed from a passing run.

**Comment findings.** The gate (`commented-code`) is clean. The advisory report
flagged exactly one finding introduced by this diff — a `[Render]` doclink that
resolves to nothing — fixed to `[Renderer.Render]` rather than left to grow the
backlog. No suppressions added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 15 total across design review and code review.

Design review (8, all addressed before implementation): RR-7BYA4X, RR-S0I1WH,
RR-CC6VEW, RR-S4I9M9, RR-CVTBSK, RR-PIBARP, RR-X06ZWR, RR-RSRUXK.

Code review (7):

| ID | Sev | Status | Finding |
|---|---|---|---|
| RR-0WDY5Y | critical | addressed | Data race on `Outbox.cancel` between Start and Stop; Stop could see nil while a worker ran and leak it |
| RR-YMJOEO | critical | addressed | Text part unescaped AFTER stripping tags, re-materializing entity-encoded markup into the delivered body |
| RR-K7PZ0T | critical | addressed | Unvalidated `BaseURL` defeated the href scheme allowlist |
| RR-61UFRU | significant | addressed | Dead `Services.mailOutbox`; SVG rule documented but unenforced; empty password authenticated blindly |
| RR-TON3PU | significant | addressed | Drain timeout abandons a goroutine holding an SMTP connection — now documented rather than understated |
| RR-LW5DW6 | significant | **deferred** | Head-of-line blocking: one bad message stalls the queue ~30s |
| RR-1UEMS9 | significant | addressed | SMTP test fake leaked a goroutine per abandoned connection (intermittent goleak failure) |

All three criticals were **reproduced before fixing and re-verified after** —
none were taken on faith. The race was reproduced under `-race` with a 50-
iteration interleaving; the sanitizer bypass and the href bypass were each
demonstrated producing live markup / a `javascript:` URL in rendered output.

**One finding was partly wrong and is recorded as such.** The reviewer reported
that `stripEmphasis` corrupting `5*3*2` → `532` made the HTML and text parts
disagree. The corruption is real, but goldmark parses `5*3*2` as emphasis per
CommonMark, so the HTML part renders `5<em>3</em>2` — the parts *agree*, and both
follow the spec. Verified across four inputs and pinned by
`TestRender_PartsAgreeOnEmphasis`. The AST rewrite fixed the mechanism the
reviewer objected to without changing this output, which is correct.

**RR-LW5DW6 is deferred deliberately, not skipped.** Fixing it properly means
classifying permanent SMTP failures separately from transient ones, or
requeue-with-deadline instead of sleeping the consumer — queue semantics that
belong with the durable backend (IDEA-WIJ2H1). A partial retry-classification
would look like a fix while still serializing on transient failures. The stall
arithmetic is now written into the package doc and the operator guide.

**Self-review of the diff:** `internal/appbuild/appbuild.go` gained two
extractions (`resolveVisibleSearcher`, `startBackgroundServices`) that are not
strictly mail work. They are there because adding mail pushed `assemble` past
the `funlen` cap, and extracting was the honest fix rather than suppressing the
lint. Called out here so a reviewer is not surprised by them.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all PASS. Full evidence table in IMPL-U2YD0E, including
the decoded wire capture from the end-to-end harness. Summary:

| AC | Status | Evidence |
|---|---|---|
| 1 config absent → mail off | PASS | `TestLoadConfig_Missing`, `TestStartMail_NotConfigured` |
| 2 SMTP + mandatory STARTTLS | PASS | `TestSMTP_DeliversOverSTARTTLS` (real cert), `TestSMTP_RefusesPlaintextDowngrade` |
| 3 memory transport records | PASS | `TestMemorySender`, `TestMemorySender_RingBuffer` |
| 4 shared conformance suite | PASS | `TestConformance_Memory` + `TestConformance_SMTP`, 11 clauses |
| 5 golden-pinned render | PASS | `TestRender_Golden` |
| 6 / 6a–6d sanitize + ordering + CSS injection + palette | PASS | 5 tests incl. post-inline assertions |
| 7 text/plain always present | PASS | `TestRender_AlwaysHasTextPart` |
| 8 enqueue never dials | PASS | `TestOutbox_EnqueueDoesNotDial` |
| 9 / 9a / 9b retry, CRLF subject, bounded stop | PASS | 3 tests |
| 10 loss on restart pinned | PASS | `TestOutbox_LosesPendingOnStop` |
| 11 credential never leaks | PASS | `TestSMTP_CredentialNeverInError`; 0 occurrences in captured wire bytes |
| 12 arch-lint | PASS | `just arch-lint` → "OK - No warnings found" |

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-MAIL01

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

No TODO/FIXME in the diff. The manual end-to-end harness is `//go:build
mailmanual`, excluded from the default build, so it is not "temporary code to
remove before merge".

**Ready for another developer:** the `Sender` seam has two working
implementations and a conformance suite, so adding the HTTP and script
transports in TKT-DS1CR6 is a matter of passing `mailtest.RunAll`. Nothing
enqueues mail yet — that is TKT-U2R7GU by design, and `Message.RenderedFor` is
the documented seam it attaches to.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1401
