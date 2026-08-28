---
id: PLAN-DS1CR6
type: planning-checklist
title: 'Planning: Mail extensibility: HTTP + Lua script transports, mail.send binding, base64/multipart/basic-auth primitives'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
IN: one Go `transport: http` (SimpleMailService APIv2); `transport: script`
running an operator Lua script over an already-rendered message; general Lua
primitives `crypto.base64_encode`/`_decode`, `http` `form` (multipart/form-data)
and `basic_auth`; a `mail.send{...}` binding wired through
`lua.LoadContextOptions`; example scripts for Mailgun/Postmark/Resend in
`examples/mail/`.

OUT (unchanged from the ticket's Scope: IS NOT): no Mailgun/Postmark/Resend Go
transports, no JSON field-mapping DSL, no inbound mail / bounce handling /
open-click tracking, no durable queue (IDEA-WIJ2H1).

**Acceptance Criteria:** The ticket's ten criteria, unchanged. Test scenarios
are mapped criterion-by-criterion in the review checklist (REV-DS1CR6).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: PLAN-EQC0Q8 holds the
  feature-wide SMTP/rendering research; this slice adds no third-party library)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — no new dependency. The provider comparison in the
ticket body is the research output, and it is what rules out a mapping DSL.

**Existing Solutions:**
- `internal/mail/mailtest` (TKT-332QZY) is the conformance suite. Both new
  transports must pass it UNCHANGED; that constraint is what keeps `Sender`
  honest rather than growing a per-transport contract.
- `internal/lua/crypto.go` establishes the free-function module-registration
  shape used to keep `Runtime`'s method count flat.
- `internal/lua/http.go` and `internal/lua/ai.go` establish the
  `(nil, err_table)` convention, the `kind`/`message`/`retry_after`/`details`
  error shape, and `httpContext` for deadline propagation.
- `lua.Capabilities` (TKT-YH52OM, PR #1385) is the shipped sandbox. VERIFIED
  before building: `HTTP`/`AI`/`WriteFile` bools plus `Secrets []string`,
  fail-closed zero value, `AllSecrets` not settable from YAML. Matches what the
  ticket assumed.
- `internal/secrets.Load(relaDir, scriptPath)` already keys per-script
  overrides on the script path — which is what makes the script path the right
  secrets scope for a send script rather than a mail-only invention.
- FEAT-CN5L0X (`examples/idp-sync.lua`) is the precedent for shipping the
  generic primitive in Go and the provider specifics as example Lua.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `internal/mail/http.go`: `HTTPSender` posting the APIv2 body. Base URL is a
   constant with a Go-only test override — never a config key.
2. `internal/mail/script.go`: `ScriptSender` builds a `lua.NewReader` with a
   ZERO `ReadDeps`, the config's capability grant, secrets loaded under the
   script-path scope, and a fixed `system:mail` principal. The rendered message
   arrives as a `message` global.
3. `internal/lua/crypto.go`: `base64_encode` (raises on wrong type) and
   `base64_decode` (returns an error pair — its input is untrusted).
4. `internal/lua/http.go`: `httpRequestOpts` gains `form` and `basicAuth`;
   `doHTTPRequest` takes the struct rather than seven positional arguments;
   header parsing is deduplicated into one helper shared by `http.request` and
   the convenience methods.
5. `internal/lua/mail.go`: `registerMailModule` free function, `MailSender`
   consumer-side interface, registered unconditionally with a `not_configured`
   guard.
6. `lua.LoadContextOptions` gains a `MailSenderLoader` parameter;
   `internal/script` supplies `mail.LoadLuaSender`.
7. `mail.SenderFor` is hoisted out of `internal/appbuild` so the four-transport
   switch exists once.

Rejected:
- A JSON field-mapping DSL — the provider table shows Mailgun is not JSON, so
  the DSL would exclude a major provider by construction while still being a
  language to learn.
- A configurable `http` base URL — it would make a credential-bearing request
  redirectable from a project file.
- Importing `internal/mail` from `internal/lua` for the binding — a cycle,
  since `mail` must import `lua` for `transport: script`. The consumer-side
  interface plus a loader parameter resolves it in the sanctioned direction.
- Carrying the enqueuing principal for the send identity — see the ticket's
  decided open question.

**Files to modify:**
- `internal/mail/{http,script,config}.go` and tests (http.go, script.go new)
- `internal/lua/{crypto,http,mail,context,runtime}.go` and tests (mail.go new)
- `internal/script/runtime.go`
- `internal/appbuild/mail.go`
- `examples/mail/{README.md,mailgun,postmark,resend}.lua` (new)
- `.go-arch-lint.yml`
- `docs-project/entities/guides/{GUIDE-mail,GUIDE-lua-scripting}.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `.rela/mail.yaml`: closed transport set; `account_id` must be a plain
  identifier (it becomes a URL path segment, so a `/`, `?`, `#` or `..` could
  retarget an authenticated request); `script` must be a project-relative
  `.lua` path that does not escape the project.
- Provider responses: bounded read (4 KiB) into error messages, redacted.
- Lua `form`/`basic_auth` arguments: strings only, no numeric coercion; an
  empty `basic_auth.user` is refused so a mistyped key cannot send an
  empty-username credential.
- `crypto.base64_decode` input is untrusted by definition; its error message
  deliberately does not echo the input, which is often a credential.

**Security-Sensitive Operations:**
- The send runtime has NO graph access: zero `ReadDeps`, reader not writer.
  Asserted by `TestScriptSender_NoGraphAccess`, not stated in prose.
- Capability grant is fail-closed and narrower than `lua.Capabilities`:
  `ai` and `write_file` are hard-wired false for a mail transport.
- `secrets` is a list of key names; an ungranted key is absent, not empty.
- Credentials never appear in `Config` fields, so no config dump can carry
  them; provider errors are redacted through the existing `mail.redact`.
- A send script is trusted code (the `internal/ai` posture). Capability gating
  narrows blast radius; it does not make a hostile script safe. Stated in the
  `ScriptSender` package doc.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
Each of AC1–AC10 has a named test; the mapping is in REV-DS1CR6. The
conformance suite is run against all four transports from `internal/mail`.
The shipped `examples/mail/*.lua` are executed from `examples/` — not from
inlined copies — so a rotted example fails the build.

**Edge Cases:**
Repeated form field names (Mailgun's `to`); a colon inside a Basic-auth
password; binary round-trip through base64 across all 256 byte values; the
unpadded and URL-safe base64 alphabets; a caller-set `Content-Type` colliding
with the generated multipart boundary; `body` and `form` both set; an inner
send script calling `mail.send` (must not recurse).

**Negative Tests:**
Missing/blank/traversing/absolute/non-`.lua` script paths; missing or
separator-bearing `account_id`; a missing API token; a provider 401 echoing the
token; a send script without the `http` capability; an undeclared secret; a
malformed `mail.send` call; CR/LF in a subject reaching either new transport.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **`Sender` reshape** — did not materialize. `Sender` was already
  `Send(ctx, Message) error` with an already-rendered message, which is exactly
  what both new transports need. TKT-332QZY's sanity check held.
- **Exfiltration via a send script** — narrowed by the capability grant and the
  no-graph-deps runtime, and asserted. The "trusted code" posture is restated
  in the package doc as the ticket required.
- **Worker-context gap** — resolved; see the ticket's decided open question.
- **Example-script rot** — the shipped examples are executed against local
  stubs, which pins rela's contract and not the providers'. Said plainly in
  `examples/mail/README.md` and in the guide rather than implying support.
- **`Runtime` at its plimsoll load line** — real, and it moved 105 -> 106. The
  reason is recorded on the directive: `contextcheck` follows a registration
  closure to twelve unrelated call sites, and a method value is the same shape
  `ai.*`/`http.*` already use. `registerMailModule` remains a free function as
  specified.

**Effort:** m (as filed).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `GUIDE-mail.md` — transport table, transport-choice guidance, the `http`
  and `script` transports, credential sourcing, the secrets-scope decision,
  example-script caveat, troubleshooting
- [x] `GUIDE-lua-scripting.md` — `form`/`basic_auth`, the `crypto` module
  including base64, and `mail.send`
- [x] `examples/mail/README.md` — how to install and adapt a send script
- [x] ~~`CLAUDE.md`~~ (N/A: no repository-wide convention was added; the work
  follows the existing consumer-side-interface and single-load-point rules)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the ticket
  was filed with a researched provider comparison, explicit IS / IS NOT scope,
  ten acceptance criteria and a named open question — the design review's
  output already existed in the ticket body)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** The one open design question the ticket raised is
decided and recorded in the ticket body with its reasoning and the rejected
alternative.
