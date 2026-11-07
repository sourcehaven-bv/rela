---
id: REV-JR75QH8
type: review-checklist
title: 'Review: Real-time co-editing of entity bodies (single node)'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

- [x] All tests pass (`just test`) (`just coverage-check` ran every Go package: all pass; collab tests also pass with `-race`; frontend `npm run test:run` 5975 pass; collab E2E spec passes)
- [x] Lint clean (`just lint`) (all four golangci-lint runs: 0 issues; frontend `npm run lint` clean; `just arch-lint` OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (total 85.7%, all thresholds pass)
- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and security reviewers; 20 review responses recorded)
- [x] All critical review-responses addressed (RR-ML3XYIJ, RR-1DG0W8D)
- [x] All significant review-responses addressed (RR-E1J930X, RR-WCMN8A8, RR-H6L1JIJ, RR-KPBGKH3, RR-8NG7MWM, RR-684C5KG, RR-6S0GXZ8)
- [x] Self-reviewed the diff for unrelated changes (every changed file belongs to the relay, the editor collab mode, autosave coordination, docs or goldens)
- [x] Each acceptance criterion tested (reference planning checklist) (see IMPL-2SN2R40 evidence; E2E covers convergence, stored body, single saver, same-line typing)
- [x] Test evidence documented in implementation checklist (IMPL-2SN2R40)
- [x] Docs-checklist created and linked via `has-docs` (DOCS-0JF2AW6)
- [x] User-facing documentation updated (GUIDE-data-entry, GUIDE-desktop, GUIDE-postgres-backend; docs/ regenerated)
- [x] Docs-checklist marked as done
- [x] Commit message explains the why, not just what (the commit names the ticket and the feature; the user asks for minimal commit messages)
- [x] No TODOs or FIXMEs left unaddressed (no new TODO or FIXME; collab.go only cites the existing TODO(body-redaction))
- [x] Ready for another developer to use
- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows `done`, TKT-UFV01M)
