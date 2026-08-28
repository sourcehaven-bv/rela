---
id: REV-DS1CR6
type: review-checklist
title: 'Review: Mail extensibility: HTTP + Lua script transports, mail.send binding'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Architecture boundaries clean (`just arch-lint`)
- [x] Type load lines clean (`just plimsoll`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Markdown lint clean (`just lint-md`)
- [x] Generated docs up to date (`just docs-check`)

**Note on `just lint`.** Adding the binding surfaced twelve `contextcheck`
findings at unrelated `NewReader`/`NewWriter` call sites, because the analyzer
follows a registration CLOSURE and asks each site to thread a context into
runtime construction. Registering a method value — what `ai.*` and `http.*`
already do, and what the analyzer does not follow — removed all twelve. One
remaining finding on `rt.RunFile` is suppressed with the same reason
`internal/script` and `internal/docs` already use: the context IS threaded, via
`lua.WithContext`, across a functional option the analyzer cannot follow.

## Code Review

- [x] Self-reviewed the diff for unrelated changes
- [x] No critical review-responses outstanding
- [x] No significant review-responses outstanding

**Review Responses:** none raised.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| 1 — http against an APIv2 stub | PASS | `TestHTTPSender_APIv2Contract` asserts `POST /v2/accounts/{id}/messages`, `Authorization: Bearer`, and `from{email,name}` / `recipients[]{email,name}` / `subject` / `html_content` / `text_content`; `TestHTTPSender_InlineImagesAsAttachments` covers `attachments[]` |
| 2 — script delivers via Lua | PASS | `TestScriptSender_Delivers` |
| 3 — all four transports pass the suite UNCHANGED | PASS | `TestConformance_{SMTP,Memory,HTTP,Script}`; `internal/mail/mailtest` is untouched by this branch |
| 4 — send runtime has no graph access | PASS | `TestScriptSender_NoGraphAccess` probes read bindings (none return), mutation bindings (all absent) and an undeclared secret (nil); `TestScriptSender_UndeclaredSecretIsAbsent` isolates the secrets half |
| 5 — shipped mailgun.lua posts multipart with Basic auth | PASS | `TestExampleMailgunScript` runs the file from `examples/`, parsing real multipart and asserting Mailgun's `from`/`to`/`subject`/`html`/`text` plus repeated `to` |
| 6 — base64 round-trip with known vectors | PASS | `TestBase64EncodeKnownVectors`, `TestBase64DecodeKnownVectors` (RFC 4648 §10), `TestBase64RoundTripBinary` (all 256 bytes), `TestBase64DecodeVariants` |
| 7 — form is well-formed multipart; basic_auth header correct | PASS | `TestHTTPForm_MultipartWellFormed` (parsed, not matched), `TestHTTPForm_BasicAuthHeader` (exact header), `TestHTTPForm_RepeatedFieldNames`, `TestHTTPForm_MapShapeIsSorted` |
| 8 — mail.send works with the grant; fails loudly without | PASS | `TestMailSend_Succeeds`; `TestScriptSender_NoHTTPCapabilityIsLoud` (denied http fails the send rather than no-opping); `TestMailSend_RegisteredWithoutSender` (present-and-typed, not silently absent) |
| 9 — script failure is a typed err_table, retries without duplicating | PASS | `TestMailSend_DeliveryFailureIsErrTable`; `TestScriptSender_FailureIsRetryableAndDoesNotDuplicate` (one request per attempt, identical bytes) |
| 10 — credential never in logs, errors or config | PASS | `TestHTTPSender_ErrorRedactsToken` (provider echoes it back; the error carries `<REDACTED>` and still names the 401); `TestHTTPSender_DoesNotLeakTokenInConfigDump` (the credential is not a `Config` field, so no dump can carry it) |

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DS1CR6

## Final Checks

- [x] Commit messages explain the why, not just the what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` to create the PR and monitor CI~~ (deferred: the PR is opened
  after this checklist is done, so its URL and CI result post-date it —
  see TKT-UFV01M)
