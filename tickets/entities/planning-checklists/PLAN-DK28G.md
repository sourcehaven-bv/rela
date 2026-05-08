---
id: PLAN-DK28G
type: planning-checklist
title: 'Planning: Quick-search/jump command palette for data-entry UI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: a global `Cmd+K` / `Ctrl+K` modal that searches entities by
title/ID/type, with up/down/Enter navigation and Esc/click-outside dismissal.
Reachable from every data-entry route. Closes when an entity is selected and
routes to that entity's detail view (respecting custom detail views).

Out of scope: command actions (create/toggle), recents/pinned entries,
dashboard/view targets, server-side ranking changes, customizable keybinding.

**Acceptance Criteria:**

See ticket TKT-77JD4 for the canonical AC1–AC10 list. Each is mapped to a test
scenario in the Test Plan section below.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

See full list in earlier revision; key reuse: existing `searchEntities` endpoint
(extended with optional `signal: AbortSignal`), `entityDetailHref`,
`useModalStack`, `isCancelledFetch` from `usePageData.ts`. No new dependencies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Implemented as `frontend/src/components/ui/CommandPaletteModal.vue` mirroring
the ConfirmModal pattern (Teleport, modalStack registration, focus restore on
close, Escape stopPropagation). `paletteOpen` ref lives in
`useKeyboardShortcuts` and is flipped on Cmd/Ctrl+K. Search uses an
AbortController per request, debounced at 150ms, with a 2-char minimum and
client-side cap at 50 results. ARIA combobox/listbox pattern with
`aria-activedescendant`.

**Files modified:**

- `frontend/src/composables/useKeyboardShortcuts.ts` — added `paletteOpen` ref + Cmd+K handler + `isAnyModalOpen()` short-circuit for non-trigger shortcuts.
- `frontend/src/composables/useKeyboardShortcuts.test.ts` — extended.
- `frontend/src/composables/index.ts` — re-exported `paletteOpen`.
- `frontend/src/api/entities.ts` — extended `searchEntities` with optional `signal: AbortSignal` (with JSDoc).
- `frontend/src/components/ui/CommandPaletteModal.vue` — new component (~270 lines).
- `frontend/src/components/ui/CommandPaletteModal.test.ts` — new test file (34 tests).
- `frontend/src/components/ui/KeyboardShortcutsModal.vue` — added Cmd/Ctrl+K row to Global section.
- `frontend/src/App.vue` — mounted `<CommandPaletteModal>` next to `ConfirmModal`.

**Dependencies:** None new.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Search query (user input):** sent verbatim as `q=` to the existing `/api/v1/_search` endpoint (already in production for the standalone Search page). Result fields are rendered as text via `{{ }}`, never `v-html`, so XSS via title/ID is not a concern.
- **Entity ID/type from results:** passed to `entityDetailHref` which returns `''` for missing type/id; `selectEntity` early-returns on empty href before `router.push`.

**Security-Sensitive Operations:** None. Read-only navigation aid.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

All 10 ACs have mapped test scenarios. 34 component tests + 4 composable tests
cover happy path, debounce, AbortController forwarding, race conditions,
in-flight refetch flicker prevention, custom detail views, idempotency, Tab
trap, Escape stopPropagation, modal-stack integration, MAX_RESULTS cap,
single-character query short-circuit, unmount cleanup.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** **m**. Final scope ended up slightly larger than estimated due to
the cranky review round (added `isAnyModalOpen` gate, MAX_RESULTS cap, unmount
cleanup) — still a single-day implementation.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] In-app shortcuts modal (`KeyboardShortcutsModal.vue`) — new row for Cmd+K (Quick jump)
- [x] N/A: User guide / reference docs (no separate SPA user guide)
- [x] N/A: CLI help text (no CLI surface)
- [x] N/A: CLAUDE.md (no new patterns)
- [x] N/A: README.md (no project-level changes)
- [x] N/A: API docs (no API change — `searchEntities` signature change is frontend-internal)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (pre-implementation):**

| RR | Severity | Title | Status |
|----|----------|-------|--------|
| RR-4LHM6 | significant | Title field name: title vs _title | addressed |
| RR-MP29E | significant | Escape must stopPropagation | addressed |
| RR-HTS2Q | significant | Cmd+K idempotency when already open | addressed |
| RR-2H6YE | significant | Tab key leaks focus to background | addressed |
| RR-GMZZP | minor | Empty query must not call /_search | addressed |
| RR-9IHU2 | minor | Prefer AbortController | addressed |
| RR-R51D0 | minor | Don't blank results during refetch | addressed |
| RR-QL4SD | minor | aria-activedescendant + per-row id | addressed |
| RR-WQRYA | nit | Naming: command palette vs quick jump | addressed (user chose "Quick jump") |
