---
id: TKT-GL18KOA
type: ticket
title: 'Lint: no comments in function bodies (Go and TS)'
kind: chore
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

Add a lint gate for Go and TypeScript that rejects comments inside function
bodies, so the author renames, extracts a helper or names a constant instead.

## Scope (v1)

- Flag `//` and `/* */` comments inside function bodies (Go funcs and methods, TS functions, arrows, methods).
- Directives are always allowed: Go `//go:`, `//nolint`, `//plimsoll:`, `//commentlint:`, `//coverage-ignore`, `//lint:`; TS `eslint-`, `@ts-`, `istanbul`, `c8`, `prettier-ignore`, `@vitest-`.
- Limit the comment on a function: a doc comment above a func is capped at 10 lines. Corpus (non-test Go, ~13.9k documented funcs): 9% exceed 10 lines, so the cap starts a bounded worklist.
- Gate only new and changed lines; the existing comments become a worklist.
- Tests exempt. Declarations outside function bodies unaffected.

## Decision

Cap is on doc-comment length per function (confirmed by the user).

## Threat model

No runtime surface. The linter reads source only and runs in CI. Risks: the
suppression marker becomes a reflex (mitigated by requiring a reason after
`why:`), and a gate red on arrival gets ignored (mitigated by gating changed
lines only).

## Design

- Go: new rule in commentlint (external repo, release + version bump in justfile and ci.yml) or an analyzer under tools/.
- TS: ESLint rule in frontend/eslint.config.js.
- Failure message: rename the function or variable, extract a helper, or name the constant.

## Design review outcome

- Gate: committed baseline that may only shrink (pattern: archguard bareref_test). Key: file + enclosing function + comment count. No line numbers, no diff logic.
- Body comments: closures count as body. Skipped: generated files, cgo preambles, `/// <reference`, tag-only JSDoc, test files. Vue: `<script>` only.
- Doc cap: 10 prose lines per function; directive lines and bare `//` separators do not count.
- Escape marker `why: <reason>`: reason text required, counted in the baseline.
- Go: in-repo analyzer under tools/ with its own tests. TS: local ESLint rule in frontend/eslint.config.js.
- Add `.claude/rules/code-comments.md` and a row in the CLAUDE.md rules table.
- Acceptance: testdata cases for each exemption; baseline fails on a new comment and passes after removal; `just ci` runs both checks.
