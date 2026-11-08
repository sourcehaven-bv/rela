---
id: PLAN-G2ZYK94
type: planning-checklist
title: 'Planning: Help texts for every form placement'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** OpenVWR shows help texts in more form placements than rela forms
support. The OpenVWR v2 port needs each placement to carry over. The inventory
maps 15 OpenVWR placement kinds onto rela; most already have a rela key.

**Scope - IS in scope:**

- Field and relation `hint` (info button with a Markdown panel).
- Relation `help` accepted by the Go config.
- Step `details` (collapsed Markdown below the step description).
- Form `description` rendered below the heading.
- Markdown rendering for field/relation `help`, step `description` and form
`description`.
- Relation cards fall back to the relation type description.

**IS NOT in scope:**

- Per-option descriptions for radio/enum values (enum `descriptions` stay off
the wire; follow-up).
- Form callouts that depend on form state (needs a non-property form item with
`visible_when`; follow-up).

**Acceptance Criteria:**

1. `hint`, relation `help`/`hint` and step `details` survive YAML to JSON.
*Test:* `TestFormHelpKeys_YAMLAndJSON`.
2. A field with `hint` shows an info button; clicking toggles a Markdown panel
with `aria-expanded`/`aria-controls`. *Test:* `FieldShell.test.ts`.
3. Relation picker and relation cards show `hint`, and `help` falling back to
the relation type description. *Test:* `RelationPicker.test.ts`,
`RelationCards.test.ts`.
4. Form `description`, step `description` and step `details` render as
Markdown; `details` is collapsed. *Test:* `DynamicForm.help.test.ts`.
5. Script tags in help Markdown are stripped. *Test:* `FieldShell.test.ts`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: mapping of existing placements; inventory in the side-session notes)
- [x] ~~Searched for existing libraries~~ (N/A: reuses the existing `renderMarkdown` and lucide icons)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- OpenVWR Filament `helperText`, `hintIcon`, `hintAction` and the
InformationBlockSection per wizard step are the reference.
- `frontend/src/utils/markdown.ts` `renderMarkdown` (DOMPurify) is reused.
- Field `help` already fell back to the property description; relation picker
already fell back to the relation type description.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add optional string keys to `FormField`, `FormRelation`
and `FormStep` in `internal/dataentryconfig`. In the SPA, add `MarkdownHelp.vue`
(sanitized Markdown) and `FieldHint.vue` (toggle button and panel), and use them
in `FieldShell`, `RelationPicker`, `RelationCards` and `DynamicForm`. The
configedit allowlist already covers the new keys through its wildcards.

**Alternatives rejected:** a hover tooltip for `hint` (not reachable by keyboard
or touch, and too small for long texts); a modal (heavier, and loses the form
context); a separate `markdown: true` flag (two ways to say the same thing).

**Files to modify:** `internal/dataentryconfig/config.go`,
`frontend/src/types/config.ts`, `frontend/src/components/forms/*`, i18n en/nl,
`docs-project/entities/guides/GUIDE-data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Help texts come from operator-authored
`data-entry.yaml`. They are rendered with `renderMarkdown`, which sanitizes with
DOMPurify's allowlist before `v-html`.

**Security-Sensitive Operations:** `v-html` in `MarkdownHelp.vue` only; no new
endpoint, write path or ACL change.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: config round trip plus component mounts cover the path; no server behaviour changes)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** no `hint` renders no button; empty `help` falls back to the
property or relation type description; step `details` resets when the step
changes; a checkbox field shows the hint next to its inline label.

**Negative Tests:** script in help Markdown is not rendered.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** plain-text help that contains Markdown characters now renders
differently (low; help texts are short prose). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `docs/data-entry.md` (generated from
`GUIDE-data-entry`).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the change was implemented before the ticket entered planning; `/code-review` in the review phase covers the design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run, see above)

**Design Review Findings:** none
