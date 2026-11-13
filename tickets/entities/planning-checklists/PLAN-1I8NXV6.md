---
id: PLAN-1I8NXV6
type: planning-checklist
title: 'Planning: Refuse remote credentials outside remotes.yaml'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: every operator config that names a credential source other than
remotes.yaml refuses a remote credential, as a secret (`remote_credential.*`) or
as an environment variable (`REMOTE_CREDENTIAL_*`). Assess RR-T2DQV3D. Out: the
file source (no other config names a credential file); sync-correctness findings
(separate ticket).

Surface inventory (secret names): subscriptions auth.secret, connections.yaml
client_id_secret/client_secret_secret, flows webhook secret, schema/mail
capabilities.secrets, Lua secret filter. All refuse since #2165, each with a
test. AI and mail secret keys are fixed names. Env names: ai.yaml api_key_env
and mail.yaml password_env accept any name: the gap. Command `env:` maps set
literal values, they read nothing.

**Acceptance Criteria:**
1. ai.yaml with api_key_env REMOTE_CREDENTIAL_X fails ParseConfig and NewOpenAICompatProvider.
2. mail.yaml with password_env REMOTE_CREDENTIAL_X fails Validate and NewSMTPSender.
3. Case-insensitive match (Windows env), bare prefix refused.
4. RR-T2DQV3D assessed and closed with a reason.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small validation change)
- [x] ~~Searched for existing libraries~~ (N/A: one prefix check)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal policy)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** secrets.IsRemoteCredential / ScriptReadable
(internal/secrets/secrets.go); httpremote.CredentialEnvPrefix
(internal/reconclient/httpremote/credential.go); subscription Auth.validate
refusal.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Move the env prefix into the leaf package secrets
(RemoteCredentialEnvPrefix, IsRemoteCredentialEnv, case-insensitive).
httpremote.CredentialEnvPrefix aliases it. ai.Config.Validate and
mail.Config.Validate refuse a match. Both sender/provider constructors already
run Validate, as does the desktop settings check. Alternative rejected:
duplicate the constant in ai with an agreement test (tokenstore pattern). ai
gets a deps entry on the leaf secrets instead, one source of truth.

**Files to modify:** internal/secrets/secrets.go,
internal/reconclient/httpremote/credential.go, internal/ai/config.go,
internal/mail/config.go, .go-arch-lint.yml, tests, docs-project guides
(acl-security, remote-sync, mail, lua-scripting).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator-authored ai.yaml and mail.yaml. A name
under the prefix is refused at load. A blocklist of one marked namespace is the
design: the remote config uses an allowlist (only the marked names), and
everything else blocks those names.

**Security-Sensitive Operations:** Reading an env var and sending it to an
operator-chosen host. Errors name the variable, never its value.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: TestParseConfig_RefusesRemoteCredentialEnv,
TestNewProvider_RefusesRemoteCredentialEnv. AC2:
TestConfig_RefusesRemoteCredentialEnv (Validate and NewSMTPSender). AC3:
TestIsRemoteCredentialEnv. Existing per-surface tests for secret names stay.

**Edge Cases:** Lowercase name, bare prefix, empty name (allowed: no auth), name
containing the prefix not at the start (allowed).

**Negative Tests:** All of the above refusals; mutation check: disabling each
check fails its test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A deployment already using a REMOTE_CREDENTIAL_ name for AI or mail
stops loading that config. That is the intended refusal; the error names
remotes.yaml. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-acl-security, GUIDE-remote-sync, GUIDE-mail,
GUIDE-lua-scripting.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None (inline design review: change is a load-time
refusal in two config validators, reusing the secrets leaf).
