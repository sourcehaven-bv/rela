---
id: PLAN-MT7W2M
type: planning-checklist
title: 'Planning: Reject cleartext http:// for plantuml_server_url except loopback'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: scheme validation for `app.plantuml_server_url` in `validateApp`; a loopback
host predicate; tests; operator docs for the knob (currently undocumented).

OUT: the SPA's rendering path (unchanged); mixed-content behaviour (the browser
already blocks an http image on an https page — that is a separate concern and
not what this validates); any other URL-valued config key.

**Acceptance Criteria:**

1. AC1 `https://` any host stays valid → `https://plantuml.example.com`.
2. AC2 `http://` loopback stays valid → `localhost`, `LOCALHOST:8080`,
`127.0.0.1:8080`, `127.1.2.3:8080`, `[::1]:8080`.
3. AC3 `http://` non-loopback rejected → `plantuml.example.com`,
`93.184.216.34`, `192.168.1.10:8080`.
4. AC4 Lookalike hosts rejected → `localhost.evil.com`, `127.0.0.1.evil.com`,
`localhost@evil.com`, `evil-localhost.com`.
5. AC5 Existing scheme/host errors unchanged → `javascript:`, `data:`,
`//evil.example`, `evil.example/x`, `https://`.
6. AC6 Rule documented in the data-entry guide's App section.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (single-predicate config validation)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — xs change, one predicate.

**Existing Solutions:**

- `net.IP.IsLoopback` (stdlib) covers all of `127.0.0.0/8` and `::1`. Chosen over
hand-comparing `"127.0.0.1"` / `"::1"`, which misses the rest of `127/8`.
- `net/url`'s `URL.Hostname()` strips port, userinfo and IPv6 brackets. Chosen
over string splitting on `:` — that mishandles both IPv6 and userinfo, and
userinfo is exactly how `http://localhost@evil.com` disguises its real host.
- Prior art in this repo for the "reject at load time, not at use time" shape:
the existing scheme check in the same function (`validate.go`), which already
rejects `javascript:`/`data:` before they can reach an `<img src>`.
- The confidentiality argument is already made in-tree at
`internal/dataentryconfig/config.go:170` (why the public plantuml.com server is
deliberately not the default). This change extends the same reasoning from
*which* server to *how* the source travels there.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add one `case` to the existing `switch` in `validateApp`, after the scheme and
host checks so their messages keep priority:

```go
case u.Scheme == "http" && !isLoopbackHost(u.Hostname()):
```

`isLoopbackHost` parses the host as an IP first and defers to `IsLoopback`;
otherwise it matches the literal name `localhost` case-insensitively. An exact
match, not a prefix/suffix/contains test — those accept `localhost.evil.com`.

A name that merely resolves to 127.0.0.1 today is treated as remote: validation
runs at config load, and DNS can change afterwards.

**Alternatives rejected:**

- *Blanket https-only.* Breaks the standard local-sidecar deployment for no
confidentiality gain — on loopback there is no segment to eavesdrop.
- *Exempt RFC1918 too.* A private range is a real network segment with other
hosts on it; "private" says nothing about who can see the wire, which is the LAN
case CONTROL-8-21 is about. An operator with a private-network server can still
use TLS. Loopback is the only case where the argument truly does not apply.
- *Warn instead of reject.* The existing knob already hard-rejects bad schemes;
a warning for the confidentiality case would be inconsistent and ignorable.

**Files to modify:**

- `internal/dataentryconfig/validate.go` — the new case + `isLoopbackHost`
- `internal/dataentryconfig/validate_test.go` — table cases
- `internal/dataentryconfig/config.go` — extend the godoc rationale
- `docs-project/entities/guides/GUIDE-data-entry.md` — document the knob
(regenerate `docs/` via `just docs`; never edit `docs/` directly)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Source: `app.plantuml_server_url` in the operator's `data-entry.yaml`. Trusted
authorship (operator-audited file), so this is not an injection boundary — it is
a misconfiguration guard.

Allowlist, not blocklist: the accepted set is {any https host} ∪ {http +
loopback}. Everything else is refused. Invalid input fails config load with a
message naming the offending host, so it cannot reach a browser.

**Security-Sensitive Operations:**

The value is published to the SPA via `/api/v1/_config` and becomes an `<img
src>` carrying deflate+base64 diagram source in the URL. Two exposures: the
destination (already argued in the godoc) and the transport (this change).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (config-load validation; the table test is the integration point)

**Test Scenarios:**

Table-driven cases in `TestValidateApp_PlantUMLServerURL`, one row per AC1-AC5
input listed above. AC6 is verified by the generated `docs/data-entry.md` diff.

Both directions were mutation-tested:
1. Disabling the new case → the 7 rejection rows redden, loopback rows stay green.
2. Replacing the exact match with a naive `Contains`/`HasPrefix` → exactly the 3
lookalike-host rows redden.

**Edge Cases:**

- Empty value → valid (feature disabled); unchanged.
- `127.1.2.3` → valid; inside `127/8` but not the address people write.
- `[::1]:8080` → valid; brackets must be stripped before parsing as an IP.
- `LOCALHOST` → valid; hostnames are case-insensitive.
- `localhost@evil.com` → rejected; userinfo, real host is `evil.com`.
- `localhost.evil.com`, `127.0.0.1.evil.com`, `evil-localhost.com` → rejected.
- A hostname resolving to 127.0.0.1 → rejected; resolution is not stable and
validation happens at load time.

**Negative Tests:**

Each rejection returns a config-load error naming the host and the reason
("sends diagram source in cleartext"), so the operator can act on it.
Pre-existing scheme/host errors keep their original messages and priority in the
switch.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *An existing deployment uses `http://` to a remote server and now fails to
start.* This is the intended behaviour — but it is a breaking config change, so
the error names the fix. Mitigated by the fact that a full repo grep found
**no** YAML config, fixture, testdata or example that sets the key at all.
- *Over-refusal breaking a valid local setup.* Covered by the 5 loopback-valid
rows plus mutation 2, which proves those rows fail if the predicate is wrong.

**Effort:** xs

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — App section. `plantuml_server_url` was previously
**undocumented entirely**; this adds the knob, the trust argument for choosing a
server, and the http/loopback rule. Edited at source in
`docs-project/entities/guides/GUIDE-data-entry.md` and regenerated.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (xs config-validation change)
- [x] ~~All critical/significant findings addressed in plan~~ (no design review run)

**Design Review Findings:** N/A — xs change.
