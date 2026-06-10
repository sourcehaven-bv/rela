---
id: REV-GZCM0G
type: review-checklist
title: 'Review: Tracked vite.config.js shadows vite.config.ts — fresh-clone dev server breaks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (typecheck clean; 987 unit tests; `npm run build` and `npm run build:e2e` both leave the work tree clean)
- [x] Lint clean (0 errors, 77-warning baseline)
- [x] ~~Coverage maintained~~ (N/A: frontend coverage ratchet removed in PR #944; config-only change regardless)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer on the focused diff: 0 critical, 0 significant, 2 minor, 2 nit — reviewer empirically verified the outDir redirect with a real `vue-tsc -b` run and audited every CI job step for guard false-positives)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4K2S2Z (addressed), RR-7ALB6A (addressed — also covers
both nits: collapsed guard + dedup'd gitignore)

## Acceptance Verification

- [x] Each acceptance criterion tested (artifacts deleted from git; emit verified to land in node_modules/.cache/tsc-node; vite resolves vite.config.ts as the only config; reviewer confirmed Vite never scans node_modules for configs)
- [x] Test evidence documented in implementation checklist (IMPL-8QVXRY)

**Acceptance Status:** PASS — both build modes (including build:e2e, the command
that regenerated the artifacts twice during the review work) leave `git status
--porcelain` clean; the CI tripwire fails any future PR whose frontend job
mutates the repo.

## Documentation (enhancements only)

- [x] ~~Docs section~~ (N/A: bug fix; rationale documented in frontend/.gitignore and the CI step comment)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (verified after push)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/962 (stacked on
https://github.com/sourcehaven-bv/rela/pull/960)
