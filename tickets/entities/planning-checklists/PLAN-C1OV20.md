---
id: PLAN-C1OV20
type: planning-checklist
title: 'Planning: Replace emoji with an SVG icon set; add icon: to kanban columns and swimlanes'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Split note.** PR 3 of 3 from TKT-5V8704's design review (FEAT-OJ8L0H).
> Full research and the five design-review findings live in PLAN-J86M7L;
> RR-GWVGDX (critical) and RR-09N4MN govern this ticket.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** replace emoji with a real SVG icon set in the sidebar, and give
kanban columns/swimlanes a proper `icon:` field. OUT: tokens (PR 1), layout (PR
2), label humanization, and the emoji in other components (EntityList,
CommandModal, InaccessibleField, StatusBar) — those are separate surfaces the
ticket doesn't cover.

## Research

- [x] ~~Run `/research`~~ (N/A: library choice is narrow and documented below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Library:** Lucide (`lucide-vue-next` 1.0.0, ISC) — tree-shakeable named
imports, self-hosted (mandatory: the SPA is embedded in a Go binary and must not
fetch icons at runtime), `currentColor` by default.

**font-awesome 4.7 rejected** even though it is already a dependency: it is
present only because EasyMDE's toolbar needs it, and v4 is an icon *font* — no
tree-shaking, ships the whole webfont, no per-path `currentColor`.

**Prior art:** `TestAppTokensCSSInSyncWithFrontend` pins a Go copy of a frontend
file; the icon allowlist test follows that pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

`utils/icons.ts` is the single boundary a config string crosses to become a
component — a static allowlist, never dynamic resolution. `ValidIconNames`
mirrors it Go-side so an unknown name fails at config load.

**Rejected:** parsing emoji out of user `label:` text to convert existing
configs automatically. That is a lossy heuristic over user data and silently
rewrites what an author typed (RR-GWVGDX). Authors migrate explicitly; an emoji
left in a label keeps rendering verbatim.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

Icon names come from project-authored YAML and select a **component**, so the
lookup is an allowlist by construction. `resolveIcon` uses an own-property check
rather than a bare index: `ICONS['toString']` would otherwise return an
inherited function, which is not a component and would crash the render. A test
covers `toString`/`constructor`. No new network surface — icons are bundled, not
fetched.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

1. *Allowlist drift* — `TestIconAllowlistMatchesFrontend` parses the real
`icons.ts` and compares both directions.
2. *Unknown name* — `validateIconName` rejects with an indexed message listing
valid names; `resolveIcon` falls back without throwing.
3. *Prototype pollution* — `toString`, `constructor` resolve to the default.
4. *Rendered output* — browser check that icons are `<svg>`, not text.
5. *Theme* — icon stroke matches its label colour in both themes.
6. *Regression* — full frontend + Go suites, lint, typecheck, docs-check.

**Edge cases:** empty icon (means "no icon", valid); an emoji left in a label
(renders verbatim, never stripped); a column with no icon (renders label only).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

- *New runtime dependency.* Mitigated: ISC, tree-shaken, self-hosted; bundle
delta checked via the build.
- *Two allowlists drifting apart.* Mitigated by the cross-language test.
- *Config surface is permanent.* `icon:` is one optional string following the
same shape as the `span:` key added in PR 2.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

- [x] `docs/data-entry.md` — **required**, this adds a public config key.
Authored in `docs-project/entities/guides/GUIDE-data-entry.md` (the generated
file is not the source — a lesson from PR 2's CI failure).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** inherited from TKT-5V8704's review. RR-GWVGDX
(critical) established that kanban emoji live in user-authored label strings and
must not be parsed out — this ticket's `icon:` field is that finding's
resolution. RR-09N4MN identified the five sidebar emoji outside the
`getIconEmoji` map that a naive swap would have missed.
