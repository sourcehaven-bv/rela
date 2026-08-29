---
id: IMPL-DS1CR6
type: implementation-checklist
title: 'Implementation: Mail extensibility: HTTP + Lua script transports, mail.send binding'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration boundaries exercised through the mailtest conformance suite
      and the appbuild transport switch
- [x] Happy path implemented
- [x] Missing credentials, denied capabilities, undeclared secrets, provider
      non-2xx, malformed config and script failures all handled explicitly
- [x] Errors surfaced with transport and script-path context, credentials
      redacted

## Test Quality

- [x] Tests assert on the PROVIDER's field names, spelled out literally rather
      than derived from Go struct tags — a test reading the tags would pass
      after a rename that breaks delivery
- [x] The multipart body is parsed with `mime/multipart`, not string-matched, so
      a mismatched boundary or missing terminator fails
- [x] The shipped `examples/mail/*.lua` are executed from `examples/`, not from
      inlined copies, so a rotted example fails the build
- [x] The no-graph-access property is probed binding-by-binding from inside a
      real send runtime and reported back over HTTP, distinguishing "raised",
      "absent" and "RETURNED"
- [x] Base64 uses the RFC 4648 §10 vectors plus an all-256-bytes round trip

## Manual Verification

- [x] Feature manually tested end-to-end against `httptest` stubs for all four
      transports
- [x] Each acceptance criterion verified against the final implementation

**Verification Evidence:** `just test` (race detector, shuffled) passes with no
failures. All four transports — smtp, memory, http, script — run the TKT-332QZY
conformance suite unchanged: `TestConformance_SMTP`, `TestConformance_Memory`,
`TestConformance_HTTP`, `TestConformance_Script`. The HTTP transport is asserted
against the exact APIv2 path, bearer header and body field names; the script
transport against a stub; the shipped Mailgun example against a stub parsing
real multipart with Basic auth and repeated `to` fields.

## Quality

- [x] Follows the existing `Sender`, capability, secrets and load-point seams
- [x] `registerMailModule` is a free function per the ticket (crypto.go style)
- [x] No second mail-loading call site: `lua.LoadContextOptions` stays the
      single load point, gaining a loader parameter rather than a sibling
- [x] The transport switch exists ONCE (`mail.SenderFor`), not duplicated in
      appbuild
- [x] No JSON field-mapping DSL, no provider Go transports beyond the one
      specified, no inbound/bounce/tracking, no durable queue — Scope: IS NOT
      respected
- [x] `.go-arch-lint.yml` updated with the two new edges (mail -> lua,
      mail -> principal, script -> mail), each carrying its reason
- [x] No debug code left behind

**Deviation recorded:** `Runtime`'s plimsoll ceiling moved 105 -> 106. The
binding had to be a method value because `contextcheck` follows a registration
closure back to twelve `NewReader`/`NewWriter` call sites and demands a context
be threaded into runtime construction, which is meaningless for a binding
invoked later by a script. `ai.*` and `http.*` register method values for
exactly this reason. The registration function itself remains free, as
specified. The reasoning is on the directive.
