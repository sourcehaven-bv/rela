---
id: PLAN-MGV20IX
type: planning-checklist
title: 'Planning: Point to RELA_DATAENTRY_USER on acl_unstamped_principal'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

rela-server's default identity source is the header chain
(`$RELA_DATAENTRY_USER`, then `--principal-header`, then `unknown`). With an
`acl.yaml` and neither source set, every caller is `unknown`,
`acl.ErrUnstampedPrincipal` fires, and every `/api/` request answers 500
`acl_unstamped_principal` with the detail "principal stamper produced an
unstamped identity; check server logs". Nothing names the setting that fixes it,
and the server starts without comment.

In:
- Error detail of `acl_unstamped_principal` (internal/dataentry/router.go) names the fixes: `$RELA_DATAENTRY_USER`, `--principal-header`, or `--jwt-*`, and docs/server-security.md.
- rela-server logs a startup warning when an `acl.yaml` policy is loaded, identity mode is header, and neither `--principal-header` nor `$RELA_DATAENTRY_USER` is set. The warning repeats on each config reload that builds a new generation, since acl.yaml can appear on reload.
- GUIDE-server-security (data-entry user attribution) and GUIDE-acl-security (fails loud inside /api/) describe the warning and the hint.

Out: changing the 500 status; a warning when `--principal-header` is set but a
request lacks the header (a proxy problem per request, which the error hint
covers); rela-desktop, which always has a user.

**Acceptance Criteria:**

1. With ACL configured and an unstamped principal, `/api/v1/...` still answers 500 `acl_unstamped_principal`, and the detail mentions `RELA_DATAENTRY_USER` and `--principal-header` (extend TestACLMiddleware_FailLoudOnApi).
2. The startup warning is produced exactly when: ACL policy present, header mode, no principal header, no env user (table test over a pure function in cmd/rela-server).
3. No warning in JWT mode, with `--principal-header`, with `$RELA_DATAENTRY_USER`, or without acl.yaml.
4. Manual: rela-server on a project with acl.yaml and no identity logs the warning; a curl to /api/v1 shows the hint.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: message text)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `validateIdentityFlags` (cmd/rela-server/main.go:315) is
the pure, tested identity decision; the new warning decision sits beside it in
the same style. Startup warnings use `slog.Warn` (serve.go:212, app.go
attachments scan warning).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- router.go: detail becomes "no user identity for this request: configure --jwt-* identity (or --principal-header behind an authenticating proxy), or set $RELA_DATAENTRY_USER for local use; see docs/server-security.md". The env var is marked local-only because every caller then acts as that user (RR-Z2RB0BE).
- main.go: `func unidentifiedACLWarning(mode identityMode, f *serverFlags, envUser string, hasACL bool) bool`, true when hasACL && mode == identityHeader && f.principalHeader == "" && envUser == "".
- serve.go `buildApp`: after `wirePrincipalResolvers`, when it returns true, `slog.Warn("acl.yaml is configured but no identity source is set: every /api/ request will fail with acl_unstamped_principal", "fix", "configure --jwt-* (or --principal-header behind an authenticating proxy), or set $RELA_DATAENTRY_USER for local use", "docs", "docs/server-security.md")`, with hasACL = `svc.ACLPolicy() != nil`.

Alternative rejected: refuse to start. Header mode fails open by documented
design, and a server whose API is down still serves the SPA shell for diagnosis
(GUIDE-acl-security); a warning matches the ticket.

**Files to modify:**
- internal/dataentry/router.go, internal/dataentry/acl_middleware_test.go
- cmd/rela-server/main.go, cmd/rela-server/serve.go, cmd/rela-server/main_identity_test.go
- docs-project/entities/guides/GUIDE-server-security.md, GUIDE-acl-security.md (+ generated docs/)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** flags and the env var, read at startup; no new
input.

**Security-Sensitive Operations:** the error body. It now names configuration
settings, which are documented and not secret (CLAUDE.md, "The configuration is
not a secret"). It still never includes the raw error, the principal value or
the env var's value (RR-372L). No behaviour change in the gate itself.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 extends TestACLMiddleware_FailLoudOnApi; AC2/AC3 table
test `TestUnidentifiedACLWarning`; AC4 manual run of rela-server.

**Edge Cases:** JWT mode with ACL (no warning: JWT gate denies unverified
requests itself); env user set with ACL (no warning).

**Negative Tests:** the four no-warning rows.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** tests matching the old detail text; grep shows only the code is
asserted. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/server-security.md and docs/acl-security.md (via GUIDE entities)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-Z2RB0BE (minor, addressed), RR-VEV5702 (nit,
addressed). No critical or significant findings.
