---
id: PLAN-8SL4RGZ
type: planning-checklist
title: 'Planning: Configurable dashboard validation card and entity header History and Add to pile'
started: "2026-11-18"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `validation:` toggle on `dashboard:` and each `dashboards:`
entry; an `header_buttons:` block with `history:` and `add_to_pile:`. Each toggle
takes `true`, `false` or `{permission: <name>}`. Out: other header buttons, list
and search "Add to pile", `piles.toggles`, the root redirect and pinned sidebar
rows (other portal gaps).

**Acceptance Criteria:**
1. No new key: the validation card, History and Add to pile show as today. Test: TestPageChrome_ResolvedPerPrincipal "default", TestPageChrome_NoDashboardShowsValidation, vitest "absent (older server)".
2. `false` hides the element from everyone. Test: Go "off hides from everyone", vitest, Playwright `validation: false`.
3. `{permission: x}` shows the element to holders only; with no acl.yaml or under --read-only it shows. Test: Go holder/non-holder/no acl cases, Playwright with and without acl.yaml.
4. Bad shapes and unknown keys fail config load; `validation:` other than true on an entity dashboard is refused. Test: TestChromeToggle_UnmarshalYAML, TestValidateConfig_PageChrome, TestValidateEntityDashboards.
5. `rela acl audit` counts the new permissions as used. Test: TestDataEntryPermissions cases.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small config switch that follows the card `permission:` pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config shape, no library)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `DashboardCard.Permission` and `permitsGatedUIElement` (internal/dataentry/views_handler.go): per-principal UX filter, shown without acl.yaml or under --read-only.
- `DashboardRef` (internal/dataentryconfig/dashboard.go): a YAML value that is a bool or another shape, refusing yes/no.
- `SidebarResponse.PilesAvailable`: the principal-scoped boot payload carries per-principal UI flags.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `dataentryconfig.ChromeToggle` (Off, Permission) with
YAML and JSON (un)marshalling. The server resolves it with
`permitsGatedUIElement`: `/_dashboard` gains `validation: bool`, `/_sidebar`
gains `header_buttons: {history, add_to_pile}`. The SPA reads only the resolved
values. `/_config` stays principal-independent.

Alternatives: two keys per element (`show_x`, `x_permission`), rejected as
noisier; evaluating permissions in the SPA, rejected (no client-side ACL);
putting the header flags on each entity response, rejected because they are per
principal, not per entity.

**Files to modify:** internal/dataentryconfig/{chrome.go,config.go,validate.go},
internal/apiwire/v1/responses.go,
internal/dataentry/{dashboard_handler.go,views_handler.go}, internal/cli/acl.go,
frontend types, schema store, Sidebar.vue, EntityDetail.vue,
useEntityPileActions.ts, DashboardView.vue, docs guide, e2e spec.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator-authored data-entry.yaml. The toggle
accepts only a bool or a mapping with a non-empty `permission` string; unknown
keys are refused.

**Security-Sensitive Operations:** None. The gates are UX filters (root
CLAUDE.md "The configuration is not a secret"). The history, pile and analyze
endpoints keep their own ACL; nothing is denied or granted at runtime by these
keys.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:**
- No `dashboard:` block: validation true.
- Entity dashboard: validation always false.
- Older server without the fields: SPA shows everything.
- Unquoted `yes`/`no`: refused.

**Negative Tests:** `validation: hidden`, `{role: x}`, `{}`, `{permission: ""}`,
`header_buttons: {histroy: false}` all fail config load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Brief flash of History on a deep link before `/_sidebar` lands. Accepted: it is a UX filter and the default must stay "shown".
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via docs-project GUIDE-data-entry): dashboard `validation`, `header_buttons`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small config switch on an existing pattern; the code review covers it)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
