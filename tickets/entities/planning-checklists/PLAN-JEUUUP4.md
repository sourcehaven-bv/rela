---
id: PLAN-JEUUUP4
type: planning-checklist
title: 'Planning: Flow run panel for people who only do a step'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: step form layout, humanised names, a `manage:` flow key that
gates cancel, retry and the detailed run view, Start hidden for `per_subject:
one`. Out: harbourmaster config (its own repo), making run internals unreadable
over the API (presentation only).

**Acceptance Criteria:**
1. A customer sees a vertical step form with labelled full-width fields (e2e `expectStepFieldsStacked`).
2. A customer sees no guard, roles, diagram, timeline or cancel (e2e `expectNoManagerDetails`, FlowRunCard tests).
3. Staff with the manage permission see all of it and may cancel (e2e `expectCancelOffered`, `TestFlowControl_ManagePermission`).
4. Start is hidden while a `per_subject: one` run is unfinished (FlowRunsPanel tests, e2e).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach follows the existing `_actions` affordances)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: UI and config change within rela)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-specific)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `_actions` affordances on runs (internal/flowview), the
entity edit form layout (FieldRenderer), acl-ceiling.md for permission gating.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Optional `manage: { permission: P }` on a flow. P
authorises cancel and retry; without it the manual start permissions do, as
before. `_actions.manage` reports it, and the card shows how the run works only
then. Rejected: hiding by role names (fragile) and a runtime deny
(acl-ceiling.md).

**Files to modify:** internal/flowcfg, internal/flowview,
internal/appbuild/flowsteps.go and flowcontrol.go,
frontend/src/components/flows, i18n, e2e flows spec, GUIDE-flows.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** flows.yaml `manage.permission`: required when
`manage` is set; strict decoding rejects unknown keys; a permission no role
grants loads with a warning.

**Security-Sensitive Operations:** Cancel and retry: checked server-side with
the same permission list that feeds `_actions`. Frontend hiding is presentation
only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; appbuild tests plus Playwright e2e
for an applicant and an HR user.

**Edge Cases:** Flow without `manage`; finished run with `per_subject: one`;
runs failing to load; role held without a form; timer-only state.

**Negative Tests:** `manage` without permission fails to load; a starter without
P cannot cancel or retry.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Existing flows: unchanged behaviour without `manage`. Runs started
before `manage` was added keep the old rule (documented).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows (docs/flows.md): `manage` key, what each
reader sees, `per_subject` on /_flows, `_actions.manage`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Reviewed with cranky-code-reviewer and
rela-security-reviewer; findings recorded as review responses on the ticket.
