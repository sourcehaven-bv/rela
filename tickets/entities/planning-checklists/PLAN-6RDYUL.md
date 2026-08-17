---
id: PLAN-6RDYUL
type: planning-checklist
title: 'Planning: View section fields render as display by default; opt in to inline edit with `render: input`'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Post-implementation correction (RR-H39SEJ).** The Documentation Planning section below
> originally checked off "Changelog / upgrade notes". No `CHANGELOG.md` or upgrade-notes file
> exists in this repo and none was created — the breaking change is documented in
> `docs/data-entry.md` instead. That checkbox has been corrected to say so rather than imply an
> artifact that does not exist.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- `ViewSectionField.render: input | display` (default `display`) in `data-entry.yaml`.
- `ViewSection.render` as a section-wide default that contained fields override.
- Config validation of the enum, unconditional (RR-4ICH8M).
- Threading through the Go wire type to the SPA and into `SectionEditForm`'s per-field branch.
- Updating the e2e fixture that depends on default-editable (RR-UQ2MIV).
- The breaking-change default flip, documented in `docs/data-entry.md`.

OUT:
- `widget:` override — split to TKT-3R7RF3.
- Markdown / relation display rendering (no registry widgets exist).
- Any change to `FormField` / `DynamicForm` (the create/edit form path). Forms stay editable
by default. Confirmed with the requester.
- Side-panel inline edit: `SidePanel.vue` receives but ignores `render` (RR-4O96FZ).
- No `internal/migration` step. Breaking change accepted deliberately.

**Acceptance Criteria:**

1. A field with no `render:` renders as a display value, even when the ACL says writable.
2. `render: input` renders the editable inline widget, unchanged from today.
3. `render: input` on an ACL-read-only field still renders display. **Security-critical.**
4. Section `render:` sets the default; a field-level `render:` overrides it.
5. Invalid `render:` is a config error naming view + section index (+ field index for a
field-level value), **including when the section's source type does not
resolve**.
6. After a `render: input` field is edited, a sibling `render: display` field shows its correct
value.
7. A status/machine field with `render: display` renders the display arm, not a disabled
`StatusControl`.
8. Changing a field to display does not fire the "Permission changed" toast.
9. An all-display section does not mount a `SectionEditForm`.
10. `render` on a `display: table` / `content` section produces a validation warning naming
the mode and the precise origin (RR-675AA0, RR-1SNYI1).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the approach is fully determined by existing code.

**Existing Solutions:**

- **The display rendering already existed.** `SectionEditForm.vue` branches per field three
ways; the third arm is a bare widget in `mode="display"` with no form chrome,
already in production via ACL verdicts. This ticket feeds config into that
branch.
- **Cards/list rows already mount `SectionEditForm`** behind a tight gate (`_props`, ≤100 rows,
no inaccessible field, ≥1 writable field) — rarely satisfied, so cards usually
fall to the display path.
- **Prior art to avoid:** `FormField.readonly` → `FieldRenderer.vue:82` renders a *disabled
input*. Frontend-only; `readonly:` in YAML is silently dropped today.
- Prior tickets: TKT-IHC7B, TKT-IHC7C, TKT-IHC7D, TKT-3G93B8, BUG-9QL9XV, #997.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add `Render` to the view-section config structs, resolve it server-side, thread
it to the SPA, and use it as one conjunct on the existing `writable`
computation.

Resolution order: field → section → `display`, resolved server-side by one
shared helper (`ResolveFieldRender`) called from both builders.

Effective editability: `render === 'input' && isFieldWritable(verdict)`. Config
downgrades only; the ACL remains the authority. No new write path.

Alternatives rejected: bool `editable` (cannot grow a third mode); reusing
`FormField.readonly` (wrong semantics, wrong struct); resolving inheritance in
the SPA (duplicates the rule); splitting a view-specific wire type (RR-4O96FZ);
emitting render once per section (RR-9S63SJ).

**Files modified:** `internal/dataentryconfig/{config,validate}.go`,
`internal/dataentry/{config,sections}.go`, `internal/apiwire/v1/responses.go`,
`frontend/src/api/views.ts`,
`frontend/src/components/entity/{sectionEditFields.ts,EntityDetail.vue}`,
`frontend/src/components/forms/SectionEditForm.vue`, `e2e/tests/fixtures.ts`,
`docs/data-entry.md`, plus the two in-repo config migrations.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

`render:` is operator-authored config, validated with an **allowlist** (`input`,
`display`; empty = inherit). Invalid values fail config load with a message
naming the view, section index, field index, and the valid set. No user-supplied
input reaches this field; it never becomes a selector, path, or command.

**Security-Sensitive Operations:**

- **The ACL conjunction** — `writable = render === 'input' && isFieldWritable(verdict)`, that
order, never a disjunction. Pinned by AC 3 at both the predicate and component
level.
- **Do not pass `render` as `isFieldWritable`'s `fieldReadonly` parameter** (RR-PGGRBD).
- Write enforcement unchanged; the server re-authorizes every PATCH. Presentation-only change.
- The inaccessible (git-crypt) guard stays ahead of the render check.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Delivered:** 18 unit tests (12 frontend, 6 Go) + **4 e2e specs** in
`e2e/tests/view-section-render-mode.spec.ts` covering the mixed-render page, the
display arm's absence of form controls, the current-value guard, and the
editable opt-in (RR-5KFD7W).

**Edge Cases covered:** `render: ""` → inherit; section+field same value; no
verdict (`isFieldWritable(undefined) === true`); all-display + inaccessible;
cards/list rows via `rowShouldRouteToInlineEdit`; zero-field sections;
`title`/`id` pseudo-properties; long text.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Breaking change.** Accepted. In-repo configs migrated (28 sections). Documented in
`docs/data-entry.md`.
2. **`v1.SectionField(f)` at FOUR sites** (RR-1V04ZD) — compile-checked; all four confirmed.
3. **Spurious "Permission changed" toast** (RR-PGGRBD) — `render` kept off `verdict`. AC 8.
4. **Two builders drift** — one shared `ResolveFieldRender` helper.
5. **Long-text layout** — `isLong` + CSS ported; comment corrected per RR-1SNYI1.
6. **Test-fixture breakage** — #997 guard fixture updated; verified passing.
7. **Display-value staleness** (RR-GLK4UY, found at code review) — the default flip made a
previously-unreachable stale-mirror read the common path. Fixed with
`entryDisplayValue`.

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — new "Field Render Modes" section with before/after YAML, the
section/field option tables, the ACL rule, which display modes honour it, and a
breaking-change blockquote. Stale "read-only detail pages" line corrected.
- [x] ~~Changelog / upgrade notes~~ (N/A: this repo has no `CHANGELOG.md`, upgrade-notes, or
release-notes file — verified. The breaking change is documented in
`docs/data-entry.md`, which is where this project documents config. See
RR-H39SEJ.)
- [x] ~~docs/metamodel.md~~ (N/A: `data-entry.yaml` config, not metamodel)
- [x] ~~docs/cli-reference.md~~ (N/A: no command changes)
- [x] ~~CLAUDE.md~~ (N/A: no new architectural pattern)
- [x] ~~README.md~~ (N/A: not project-level)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-1V04ZD, RR-8EISWO, RR-UQ2MIV, RR-4ICH8M
(significant); RR-4O96FZ, RR-9S63SJ, RR-675AA0, RR-PGGRBD (minor). All
addressed.

**Code Review Findings (post-implementation):** RR-GLK4UY (critical, fixed);
RR-5KFD7W, RR-TIUKMA (significant, fixed); RR-H39SEJ (significant, wont-fix with
reason); RR-VBJ91V, RR-1SNYI1 (minor/nit, addressed).
