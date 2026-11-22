---
id: PLAN-KUV798Q
type: planning-checklist
title: 'Planning: Detail action order and danger style; hide Shortcuts on phones'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `style:` and `order:` on an action, applied to the entity page's
action buttons and its overflow menu; a `shortcuts` row in `builtins:` and a
`hidden_on_phone:` field on every `builtins:` entry. Out: action order or style
on lists, boards, the sidebar and next-action offers; moving the Shortcuts
button elsewhere; Configure editing of actions (locked as `actions.**`).

**Acceptance Criteria:** see the ticket; each criterion names its test.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: two small config keys on existing patterns)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config shape, no library)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows in-repo patterns)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `detailActions` in EntityDetail.vue sorts the `_actions['action:<id>']` keys by label; labels come from `/_config` (`Action` serialized verbatim).
- `Span` (internal/dataentryconfig/config.go) decodes an int strictly; `ActionOrder` copies it.
- `builtins:` (TKT-JEAR2UA) hides sidebar rows per space with `hidden`/`permission`, resolved by `/_sidebar` as `hidden_builtins`.
- RlSidebar turns into a drawer at `max-width: 767px`; the phone check uses the same query.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `Action.Style` (`ActionStyle`, only `danger`) and
`Action.Order` (`ActionOrder`, strict int), both with JSON tags so `/_config`
serves them. `validateActionStyle` runs for every action, the LLM branch
included. The SPA moves the sort into `offeredDetailActions` (detailActions.ts):
order, label, id. The desktop button and the `EntityAction` get `tone: danger`.

`BuiltinShortcuts` joins `BuiltinRows`; `BuiltinConfig.HiddenOnPhone` and
`Builtins.HiddenOnPhone()` feed `SidebarResponse.PhoneHiddenBuiltins`. The SPA
adds `usePhoneScreen()` (matchMedia) and `showsBuiltin` checks both sets;
SidebarFooter takes a `showShortcuts` prop.

Alternatives: the declaration order of `actions:` in data-entry.yaml, rejected
because the config decodes into a Go map and the order would need a yaml.Node
pass, and because a YAML anchor merge or a reorder for readability would then
move buttons; carrying order and style in `_actions`, rejected because that map
is a boolean affordance contract and the config is not a secret; a separate
`footer_buttons:` block, rejected because `builtins:` already models per-space
sidebar chrome.

**Files to modify:**
internal/dataentryconfig/{config.go,validate.go,builtins.go},
internal/apiwire/v1/responses.go, internal/dataentry/views_handler.go, frontend
types/config.ts, EntityDetail.vue, detailActions.ts, Sidebar.vue,
SidebarFooter.vue, usePhoneScreen.ts, GUIDE-data-entry.md (and the generated
docs/data-entry.md).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator-authored data-entry.yaml. `style:` is
an allowlist of one value; `order:` must be a whole number; `hidden_on_phone` is
a bool, and `hidden` with it is refused.

**Security-Sensitive Operations:** none. Order and style only change drawing;
the `_actions` verdict and the `POST /_action/{id}` gate are unchanged.
`hidden_on_phone` is a UX filter like the rest of `builtins:`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:**
- No `order:` anywhere: label order, as before.
- Negative order, equal order and label, missing label.
- An `_actions` key for an undeclared action: left out.
- No matchMedia (tests, old browsers): wide-screen behaviour.
- Spaces: `phone_hidden_builtins` follows the space.

**Negative Tests:** `style: primary`, `style: Danger`, `order: 1.5`, `{hidden:
true, hidden_on_phone: true}`, `{}` fail config load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- The neither-set error message for `builtins:` changes wording. Accepted:
only tests match it.
- Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via GUIDE-data-entry): action fields table, "Order
and style on the detail page", `builtins:` shortcuts row and `hidden_on_phone`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small config keys on existing patterns; the code review covers it)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
