---
id: PLAN-JX6VZC
type: planning-checklist
title: 'Planning: Design tokens: spacing, radius, typography and elevation scales for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Split note.** This ticket is PR 1 of 3 carved out of TKT-5V8704 after its
> design review. The full research, alternatives and findings live in
> **PLAN-J86M7L**; only the token-specific parts are restated here.
> Design review (5 findings, all addressed) ran on the parent before the split
> — RR-09N4MN is the one that governs this ticket.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** spacing/radius/typography/elevation scales + migration of the
high-traffic surfaces. Frontend-only, no config surface.

OUT: layout restructuring (PR 2), icons (PR 3), label humanization, and any
repo-wide CSS sweep.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: covered by the parent's
planning; this is the mechanical half.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Findings:**

- `tokens.css` is copied byte-identically into the Go binary
(`apps_tokens.css`, pinned by `TestAppTokensCSSInSyncWithFrontend`) and its doc
comment restricts it to theme tokens. New scales must go in a sibling.
- **RR-09N4MN**: the typography contract is in Go (`appTypographyCSS`,
`apps_css.go:83-95`), not `tokens.css`. `TestAppCSSSource` already existed and
asserted `--font-size-base` by name — strengthened here rather than duplicated.
- `styles/back-button.css` — precedent for a shared cross-component stylesheet
imported from `main.ts`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

New `src/styles/scales.css` imported from `main.ts` immediately after
`tokens.css`. Scales: spacing on a 4px base; radius 10 values -> 3 steps + pill
+ circle; type ramp reusing the four frozen `--font-size-*` names; three
elevation steps with a `:root.dark` override (an 8%-alpha shadow is invisible on
a dark surface).

**Alternatives rejected:** extending `tokens.css` (violates its stated boundary
and would desync the Go copy); a repo-wide regex sweep (would round deliberate
off-scale sizes).

**Decision made during implementation:** added `--font-size-md: 13px`, which is
*not* in the Go contract. 13px is the second-most-used size in the SPA (86
declarations, behind only 14px's 110) and sits inside the contract's 12->14 gap.
The contract can't absorb it without changing frozen values, so it is an
SPA-only step and does not cross the app boundary.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

No new input surface: this PR only adds CSS custom properties and rewrites
static declarations. No config, no user input, no `v-html`, no new dependency.
The one cross-boundary concern (drifting the served app stylesheet) is a
correctness risk rather than a security one, and is pinned by a test.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

1. *Contract* — `TestAppCSSSource` asserts the four `--font-size-*`
name/value pairs. **Negative test performed**: temporarily changed
`--font-size-lg` to 19px in the Go source and confirmed the test fails with the
intended message, then restored. A contract test that cannot fail is worthless,
so this was verified rather than assumed.
2. *Token resolution* — verified in-browser via `getComputedStyle`: all 15
sampled tokens resolve; `.badge` computes to 12px/4px and `input` to 14px/6px,
matching `--font-size-sm`/`--radius-sm` and `--font-size-base`/`--radius-md`.
3. *Dark override* — confirmed `--shadow-sm` differs between light
(`#00000014`) and dark (`#0000004d`).
4. *Regression* — full frontend + Go suites.

**Edge cases:** off-scale sizes left untouched by design; shorthand
`border-radius: 6px 6px 0 0`, `50%`, `em`/`rem` and existing `var()` uses
excluded from substitution (they need a human decision, not a regex).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

- *Breaking the served-app typography contract* — mitigated by reusing the
frozen names/values and strengthening `TestAppCSSSource` to assert pairs.
- *Silent visual regressions from a bulk rewrite* — mitigated by restricting
substitution to exact `property: <literal>;` declarations and verifying the
affected views in both themes in a real browser.
- *Scope creep* — mitigated by a fixed target file list; census scoped to
those surfaces.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

- [x] `frontend/CLAUDE.md` — added a "Design tokens: two files, different
contracts" section covering which file holds what, the frozen `--font-size-*`
contract, and why off-scale literals remain.
- [x] ~~docs/data-entry.md, docs/metamodel.md, CLI, README~~ (N/A: no
user-facing surface — this is internal styling with no config change.)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** inherited from the parent TKT-5V8704 review.
RR-09N4MN (minor, addressed) governs this ticket: it relocated the contract test
to the Go side and corrected the assumption that `tokens.css` held the
typography contract. The other four findings apply to PR 2 / PR 3.
