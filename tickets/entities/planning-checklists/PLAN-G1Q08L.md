---
id: PLAN-G1Q08L
type: planning-checklist
title: 'Planning: Patch brace-expansion DoS advisory in frontend devDeps (GHSA-3jxr-9vmj-r5cp)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem understood; scope = bump the transitive `brace-expansion` to patched versions.
- [x] Acceptance: `npm audit` 0 vulns; tests/lint/build unaffected.

## Research
- [x] `npm audit fix` is the non-breaking path (transitive; no `package.json` change).
- [x] `govulncheck` already clean — this is npm-side (Dependabot) only.

## Approach
- [x] `npm audit fix`; only `frontend/package-lock.json` changes.

## Security Considerations
- [x] The advisory is a DoS in devDependencies (test-utils, eslint) — never shipped. Bump removes the vulnerable versions.

## Test Plan
- [x] `npm audit` = 0 vulns; `npm run test:run`; `npm run lint`; `npm run build`.

## Risk Assessment
- [x] xs, non-breaking transitive bump.

## Documentation Planning
- [x] N/A — no user-facing docs.

## Design Review
- [x] N/A (xs dependency bump).
