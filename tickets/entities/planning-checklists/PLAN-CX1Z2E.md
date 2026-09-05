---
id: PLAN-CX1Z2E
type: planning-checklist
title: 'Planning: World chrome speaks the operator''s words or stays silent: messages, on_absent redirect, copy on_success'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: remove every rela-authored default sentence from the world chrome
(read-only note, absent banner + button, list/board notes, form face refusal
wording, badge text/tooltip, copy toast); operator text per world
(`messages.absent/projection/stand_in`), per face (`messages.read_only`) and per
copy (`on_success.message`); `on_absent.redirect` and `on_success.landing`
behaviour keys; placeholders `{face}` `{bare_face}` `{world}` `{title}`; wire
projection on `/_schema` and `_copies`; docs. Out: a general UI string table /
localisation (separate decision), address-grammar hardening (TKT-RJAPG8).

**Acceptance Criteria:**
1. With no `messages:` declared, a world-bound page, list and board show only the operator `banner:` (if any) and nothing else; the stand-in badge renders nothing; the copy toast is the copy's label.
2. Declared messages render verbatim with placeholders substituted, on the surface they belong to (atlas verify project as the fixture).
3. `on_absent: {redirect: default}` lands a reader who opens an entity with no face in the world on the target world silently.
4. `on_success.landing` `stay` / `{world}` / `{face}` navigates accordingly; `written` (default) lands on the face written.
5. An undeclared world or face in `on_absent` / `landing` is a load error.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design settled in discussion with Jeroen on 2026-09-05)
- [x] ~~Searched for existing libraries~~ (N/A)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `WorldDef.Banner` already flows operator text to the SPA
through `/_schema` (schemaworlds.go); `CopyDef.Label` flows through `_copies[]`
(copylist.go → computeCopyOffers). Both are the pattern to extend. Validation of
world/face names lives in the metamodel loader (validateWorlds /
validateCopies).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add `Messages`/`OnAbsent` to `WorldDef` (and its
UnmarshalYAML shadow), `Messages` to `FaceDef`, `OnSuccess{Message, Landing}` to
`CopyDef` with a `Landing` type accepting `written`/`stay` scalars or
`{world}`/`{face}` maps. Validate targets at load. Project onto `v1.World`,
`v1.FaceDef`, `v1.CopyOffer`. SPA: a `worldText(template, vars)` util does
`{key}` substitution; each surface reads the declared text and renders nothing
when absent; `on_absent.redirect` → `setWorld`; copy landing per offer.
Alternative rejected: a global `ui.strings` table (the right answer for
localisation, wrong granularity for per-world/per-copy variation; revisit if a
Dutch UI is wanted).

**Files to modify:** internal/metamodel/types.go, loader/validation, copies.go;
internal/apiwire/v1/responses.go; internal/dataentry/schemaworlds.go,
affordances.go (copy offers); internal/entitymanager/copylist.go; frontend:
types/schema.ts, types/entity.ts, utils/worldText.ts, EntityDetail.vue,
EntityList.vue, KanbanView.vue, DynamicForm.vue, WorldBadge.vue,
stores/schema.ts; docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator config only (schema.yaml). Messages are
plain text rendered through Vue interpolation (no v-html); placeholders are a
fixed allowlist substituted in the SPA. Redirect/landing targets validated
against declared worlds/faces at load.

**Security-Sensitive Operations:** None. Config names are not secret; no read
gate or write path changes. The redirect only changes the `?world=` query, which
the server re-authorizes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** metamodel: parse + validation tests (undeclared redirect
world, undeclared landing face, landing scalar/map forms). dataentry: `/_schema`
carries messages/on_absent/face messages; `_copies[]` carries onSuccess. SPA:
EntityDetail/EntityList/Kanban/WorldBadge/DynamicForm world suites updated to
assert silence by default and operator text when declared; redirect and landing
tests. Atlas verify project updated with Dutch messages as the integration
fixture.

**Edge Cases:** Placeholder for a face without a declared label (falls back to
the coordinate name); `{title}` on a row without a title (falls back to id);
`on_absent.redirect` to a world the reader may not read (server answers empty,
as any typed URL); redirect loop impossible (target is a declared world, never
the same page state).

**Negative Tests:** Undeclared world/face targets fail load with a named error;
unknown placeholder keys are left verbatim.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Removing default text removes the only cue on a read-only adopted
page; accepted by decision (a denial looks like any permission denial; the face
switcher remains).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - worlds/faces/copies key tables
- [x] docs/data-entry.md, docs/content-states.md - what a world-bound page shows

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design reviewed in discussion; three-option comparison recorded on the ticket)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** N/A
