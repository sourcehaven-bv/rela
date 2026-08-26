---
id: PLAN-FR2M8K
type: planning-checklist
title: 'Planning: Focus rings are a hardcoded indigo that ignores the theme, and vanish entirely in High Contrast'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — the two defects, which share one root cause (rings are hand-written
literals rather than a token):

1. The **25 live** `rgba(99, 102, 241, …)` focus-ring literals. They render an
   off-hue indigo in both themes and are unreachable by an operator's
   `custom.css` accent.
2. The **~18** `outline: none` + `box-shadow` sites that leave no focus
   indicator under `forced-colors: active`, which drops `box-shadow` entirely.

OUT — deliberately, with reasons:

- **The 47 `var(--accent-color, #6366f1)` fallbacks.** `--accent-color` is
  unconditionally defined for both themes in `tokens.css`, so the fallback is
  dead code that never renders. Churning 47 sites across 18 files for zero
  visual change buries the real fix in noise.
- **`palette.ts` / `palette.test.ts`** (16 of the 17 bare `#6366f1` hits).
  That is the default palette's accent *value*, not a hardcoded style. A
  find-and-replace here breaks palette assignment. This is the single most
  likely way to get this ticket wrong.
- **Ring geometry.** `0 0 0 2px` stays as authored. Only the colour source and
  forced-colors behaviour change; no visual redesign.
- **General accessibility.** Contrast ratios, tab order, ARIA, screen-reader
  behaviour — all out. Only the focus-ring/forced-colors interaction.

**Acceptance Criteria:**

1. A focus-ring token exists in `tokens.css`, derived from `--accent-color`,
   and follows both themes plus an operator override.
   *Test:* computed `box-shadow` on a focused control resolves to the light
   accent under `:root` and the dark accent under `:root.dark`.
2. No live indigo literal remains on a focus ring.
   *Test:* `grep -c "rgba(99, 102, 241"` over `src/` returns 0. Enforced as a
   guard test so it cannot regress.
3. Every `outline: none` + `box-shadow` control shows a real focus indicator
   under `forced-colors: active`.
   *Test:* the emitted CSS contains a `forced-colors` block restoring
   `outline` for those controls; spot-checked in-browser by forcing the media
   feature.
4. No visual change outside the ring colour.
   *Test:* full frontend suite; the `tokens.css` ↔ Go byte-sync test; manual
   check of a representative control per file group.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: mechanical sweep with a
      settled approach — the pattern was designed and shipped in TKT-CBSTYLE)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

The reference implementation is **in this repo already**:
`widgets/CheckboxWidget.vue` (TKT-CBSTYLE, [[RR-CBS2QW]] + [[RR-CBS3AC]]) does
exactly this for one widget —
`box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-color) 30%, transparent)`
plus a `@media (forced-colors: active)` block that restores
`outline: 2px solid Highlight`. It ships, so `color-mix` is proven to survive
this Vite build unmangled.

No library is appropriate: this is ~25 one-line edits plus one token.

Counted on `develop` rather than estimated — the numbers drove the scope
decisions above:

| finding | count |
| ------- | ----- |
| live `rgba(99, 102, 241, …)` | 25 |
| …of which alpha `0.1` | **21** |
| …one-off alphas (`0.25`/`0.2`/`0.06`/`0.05`) | 4 |
| dead `var(--accent-color, #6366f1)` fallbacks | 47 (out of scope) |
| `outline: none` outside tests | 21 |
| …paired with a `box-shadow` ring | 18 |
| `forced-colors` occurrences in `src/` | 1 (the CheckboxWidget fix) |

The 21/25 alpha uniformity is what makes a single token viable rather than a
per-site judgement call.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add **one** token to `tokens.css`:
   `--focus-ring: color-mix(in srgb, var(--accent-color) <a>%, transparent)`.
   Declared once in `:root` only — **verified in-browser** that the
   indirection re-resolves per theme, so no `:root.dark` duplicate is needed
   (light → `#4772fb`, dark → `#6f93ff`, both at the chosen alpha).
   Mirror byte-for-byte into `internal/dataentry/apps_tokens.css` in the SAME
   commit or `TestAppTokensCSSInSyncWithFrontend` fails.
2. Replace the 21 alpha-`0.1` sites with `var(--focus-ring)`. Handle the 4
   one-offs individually — either accept the token's alpha or keep a local
   `color-mix` at their own alpha; decide per site, do not silently flatten a
   deliberate value.
3. Add the `forced-colors` fallback. **Prefer one shared rule over 18 copies**
   — a `@media (forced-colors: active)` block in a global stylesheet targeting
   the focusable controls, rather than editing 18 scoped blocks. Falls back to
   per-file blocks only if the global selector proves too broad.
4. Add a guard test asserting no live indigo literal returns.

Alternatives considered:

- **Per-theme literal ring values in `tokens.css`** (no `color-mix`). Zero
  browser-support risk and trivially readable, but needs two declarations kept
  in sync and hard-codes an alpha blend of a colour that is itself a token.
  Rejected as the default; it is the fallback if `color-mix` causes trouble.
- **Keep `rgba()` but swap in the right hue.** Fixes the colour, not the
  theming — a literal still cannot follow an operator's `custom.css` accent.
  Rejected: it would leave the actual defect in place.
- **Also sweep the 47 dead fallbacks.** Rejected — see Scope.
- **A `:focus-visible` utility class applied across components.** The cleanest
  end state, but it edits every focusable component's markup, which is a far
  larger and riskier diff than a token swap. Better as a follow-up once the
  token exists.

**Files to modify:**
- `frontend/src/styles/tokens.css` — the token (+ Go mirror)
- `internal/dataentry/apps_tokens.css` — byte-identical copy
- ~18 component/widget files — literal → `var(--focus-ring)`
- one global stylesheet — the shared `forced-colors` block
- a test file — the no-regression guard

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: no input is read)
- [x] ~~Input validation approach defined~~ (N/A: no input is read)
- [x] ~~Security-sensitive operations identified~~ (N/A: none)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no errors)

**Input Sources & Validation:**

None. CSS token plumbing: no props, no config, no I/O, nothing interpolated
into a selector or URL.

One boundary worth naming: `tokens.css` is served to custom apps as
`_rela.css`, so adding a token **widens a public contract**. It exposes no
data — the accent colour is already published — but the name becomes something
apps may depend on, so it should read as a stable role (`--focus-ring`), not
an implementation detail.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- **AC1** (token follows theme) — already verified in-browser during planning;
  will be re-confirmed on the real build.
- **AC2** (no literals remain) — a **grep-based guard test**, which is the only
  form that actually prevents regression here. jsdom does not apply scoped SFC
  styles, so a mounted-component assertion on ring colour would pass against no
  CSS at all. TKT-CBSTYLE shipped a specificity bug underneath exactly that
  kind of green test; do not repeat it.
- **AC3** (forced-colors) — assert the built CSS contains the block, and
  spot-check by emulating the media feature in a real browser.
- **AC4** (no other visual change) — full suite + the Go byte-sync test.

**Edge Cases:**
- The 4 one-off alphas — flattening them to the token's alpha is a silent
  visual change; each needs a decision.
- `ConflictsView.vue:517` uses a bare `#6366f1` as a `border-color`, not a
  ring. Adjacent but distinct; decide explicitly rather than sweeping it in.
- `SearchBox.vue` carries `-webkit-appearance: none` — check whether it needs
  the forced-colors treatment too.
- Operator `custom.css` overriding `--accent-color` must flow through to rings;
  that is the whole point of the token.

**Negative Tests:**
- Re-introducing an `rgba(99, 102, 241, …)` ring must fail the guard test.
- The guard must not fire on `palette.ts`'s legitimate `#6366f1`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Breaking `palette.ts` with an over-broad replace.** Highest-probability
  failure. Mitigated by scoping to the `rgba(…)` ring form, never bare
  `#6366f1`, and by a guard test that must stay green on `palette.ts`.
- **`tokens.css` byte-sync.** Editing one copy and not the other fails CI.
  Mitigated by treating both as one edit.
- **A too-broad `forced-colors` selector** changing controls that were fine.
  Mitigated by scoping the shared rule to focusable controls and spot-checking.
- **Wide shallow diff → assumed-correct sites.** 25 edits across 18 files is
  exactly where "looks right, never opened it" happens. Mitigated by the guard
  test plus a per-file-group manual check.

**Effort:** s — mechanical, but 18 files and a cross-boundary token file put it
above xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] Likely **`frontend/CLAUDE.md`** — the design-token section documents the
      `tokens.css` vs `scales.css` split and its rules. A new token in the
      app-facing contract belongs there, as does "style focus rings with
      `var(--focus-ring)`, don't hand-write a box-shadow colour".
- [x] Possibly `docs/customisation.md` — if operator-facing token docs
      enumerate what `custom.css` can override.
- [x] N/A for `README.md`, `docs/metamodel.md`, `docs/cli-reference.md` — no
      CLI, schema, or project-level change.

Unlike TKT-CBSTYLE (which changed no contract), this one **adds a token to a
published surface**, so documentation is genuinely required rather than N/A.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: no
      interface, data flow, or API surface. The one design decision — token
      vs. per-theme literals vs. utility class — is recorded under Approach
      with rejection rationale, and the chosen mechanism is already shipping
      in `CheckboxWidget`)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None (review not run — see above).
