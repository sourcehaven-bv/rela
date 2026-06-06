---
id: REV-SHX0
type: review-checklist
title: 'Review: acl v1 wiring: appbuild + dataentry middleware + SSE audit + docs (PR 4)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full tree green; race-clean for
dataentry/appbuild.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Coverage maintained (`just coverage-check`) — 74.3%, package floors PASS.
- [x] arch-lint clean (`just arch-lint`).
- [x] `just docs` — regenerates docs/acl-overview.md + docs/acl-security.md
with mermaid blocks intact.
- [x] `rela --project docs-project analyze validations|cardinality` — clean.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: reference branch already
passed two cranky-code-reviewer passes producing 22 RR findings. This PR ports
the already-reviewed wiring; this checklist audits PR-4-scope findings + adds
the PR-4-original SSE audit + docs work.)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses (audit of source-branch findings applicable to PR-4 scope):**

| Finding ref | Severity | Verified in PR-4 code |
|-------------|----------|----------------------|
| RR-72OJ | critical | `appbuild.loadACLPolicy` returns a non-nil error on malformed yaml; `prepare()` propagates it; no fallback to NopACL. `TestDiscover_MalformedACL_FailsBoot` covers. |
| RR-FGJR | significant | `appbuild.Collaborators.Declarative` field; `appbuildtest.WithDeclarative` option; `TestNew_WithDeclarative_WiresBothACLAndDeclarative` pins parity. |
| RR-36UL | significant | `appbuild.WithACL` detects `*acl.Declarative` via type assertion and populates `aclDeclarative` accordingly. |
| RR-JJYW (middleware) | significant | `internal/dataentry/router.go` `attachACLRequest` middleware wraps the handler; builds Request via `Declarative.ForPrincipal`; attaches via `acl.WithRequest`. PR 3's resolver now reuses it via `acl.FromContext`. |
| RR-8ZGO | minor | `attachACLRequest` early-returns when `acl.FromContext(ctx) != nil` — respects upstream-attached Request. |
| RR-7O6Q | nit | `docs/security.md` carries the member-of self-grant hardening note; also re-articulated in `GUIDE-acl-security` for the docs site. |

**PR-4-original work (not from reference branch):**

| Item | Verified |
|------|----------|
| SSE audit-isolation invariant | godoc on `startStoreEventBridge` documents the property; `TestSSE_DoesNotFlowAuditEvents` regression test pumps a denied write and asserts zero broker events; `TestSSE_BroadcastEntityEvent_PayloadShape` pins the wire shape contains only `{type, id}`. |
| ACL docs (rela-docs entities) | `CON-authorization` + `GUIDE-acl-overview` (with mermaid concept + sequence diagrams) + `GUIDE-acl-security`; linked via `explains` + `prerequisite`; `analyze_cardinality`/`analyze_validations` both clean. Mermaid renders via existing `internal/htmlutil/mermaid.go` + frontend pipeline (no generator changes). |

## Acceptance Verification

All 7 acceptance criteria from PLAN-V0EW + TKT-YG35 verified PASS — see
IMPL-XC7S "AC mapping" for tests + results.

## Documentation (enhancements only)

- [x] Docs entities created and linked
- [x] User-facing documentation updated
- [x] Generated docs committed (docs/acl-overview.md, docs/acl-security.md)
- [x] docs/security.md member-of hardening note (RR-7O6Q)

**Docs delivered:**

- `docs-project/entities/concepts/CON-authorization.md`
- `docs-project/entities/guides/GUIDE-acl-overview.md` (with mermaid)
- `docs-project/entities/guides/GUIDE-acl-security.md`
- 3 new relation files
- `docs/acl-overview.md` + `docs/acl-security.md` (auto-generated)
- `docs/security.md` updated

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/911 (base: feat/affordances-acl-declarative)
